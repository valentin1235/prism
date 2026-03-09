# Execution Trace: API Slow Query Analysis (Without Skill)

## Request Summary
- API 응답 평균 3초 이상
- DB 슬로우쿼리 대량 발생
- `/api/v1/rooms` 엔드포인트가 특히 심각
- 최근 인덱스 관련 마이그레이션 이후 발생

---

## Analysis Approach

### Phase 1: Codebase Discovery (What I Did)

**Tools Used:** `Bash(ls)`, `Glob`, `Grep`, `Read`

1. **Project structure exploration** - Identified `podo-backend` as the relevant Spring Boot/Gradle backend project
2. **Endpoint search** - Searched for `/api/v1/rooms` or `rooms` patterns across controllers and repositories
3. **Entity/Table mapping** - Found the core domain entities:
   - `Lecture` entity -> `GT_CLASS` table (main class/lesson table, ~35+ columns)
   - `LectureOnline` entity -> `GT_CLASS_ONLINE` table (room/online session data)
   - `LectureCourse` entity -> `GT_CLASS_COURSE` table (curriculum data)
4. **Query analysis** - Examined `LectureDslRepositoryImpl` (QueryDSL) and `LectureOnlineJpaRepository` (native queries)
5. **Git history** - Checked recent 30 commits for index-related migrations (none found in recent history)
6. **Controller mapping** - Found `LectureController` at `/api/v1/lecture` with room-related endpoints like `getLeLectureRoomInfo`

### Phase 2: Findings from Code Analysis

#### Endpoint Identification
- No literal `/api/v1/rooms` endpoint was found. The closest room-related endpoints are:
  - `GET /api/v1/lecture/getLeLectureRoomInfo` - room info lookup
  - `findByRoomId(String roomId)` in `LectureDslRepositoryImpl` - joins 3 tables (lectureOnline, lecture, lectureCourse)
  - `findByRoomId(String roomId)` in `LectureOnlineJpaRepository` - JPA derived query

#### Potential Slow Query Sources Identified

1. **`findByRoomId` in DSL Repository (line 453-480):**
   - Performs a 3-way JOIN: `lectureOnline LEFT JOIN lecture LEFT JOIN lectureCourse`
   - Joins on `lectureOnline.lectureId = lecture.id` and `lecture.classCourseId = lectureCourse.id`
   - Filters by `lectureOnline.roomId` - **if ROOM_ID column on GT_CLASS_ONLINE lost its index, this full-table-scans a potentially large table**

2. **`getLemonadeLectureRoomInfo` native query (line 299):**
   - Complex native SQL with multiple JOINs, ORDER BY with TIMESTAMP function, and LIMIT 1
   - Dependent on indexes on GT_CLASS_ONLINE and GT_CLASS tables

3. **N+1 pattern in `getLemonadeLectureRoomInfo` controller (line 182-232):**
   - After fetching lecture info, makes 2 additional queries: `getLemonadeLectureAudioList` and `getLemonadeLectureVideoList`
   - 3 sequential DB calls per request

4. **Missing @Index annotations:**
   - `LectureOnline` entity has NO `@Index` annotations on `@Table`
   - `ROOM_ID`, `CLASS_ID` (lectureId), `USER_ID` columns have no declared indexes in JPA
   - `Lecture` entity (GT_CLASS) also has NO `@Index` annotations despite being queried by `STUDENT_USER_ID`, `CLASS_TICKET_ID`, `CITY`, `CLASS_TYPE`, `CLASS_STATE`, `STATUS` (CREDIT) frequently

5. **Queries without index support in `LectureDslRepositoryImpl`:**
   - `getReservedLectureList`: filters by `studentId`, `status`, `classState`, `tutorId` - needs composite index
   - `getActiveLectureList`: uses `Expressions.booleanTemplate("timestamp({0}, {1}) > sysdate()")` - **function on columns prevents index usage**
   - `getAllLectureByUserId`: full scan by `studentId` with `isPrestudy` filter
   - `getAvgClassCount`: GROUP BY on `yearMonth()` function - no index can help

6. **`LectureOnlineJpaRepository` has massive native queries (file is ~29K tokens):**
   - Many complex native SQL queries with multiple JOINs, subqueries, and ORDER BY clauses
   - `insertPodoClassOnline` uses `SYSDATE()` which can cause replication issues

### Phase 3: What I Would Do Next (If This Were a Real Investigation)

#### Step 1: Verify the Index Migration
```
# Check git log for index-related changes more broadly
git log --all --oneline --grep="index" --grep="INDEX" --grep="migration"
git log --all --oneline -- "*.sql"
```
- Look for flyway/liquibase migration files
- Check if any DDL scripts dropped indexes inadvertently

#### Step 2: Database-Side Investigation
**Tools I would use:** `mcp__grafana__find_slow_requests`, `mcp__grafana__query_loki_logs`, `mcp__mcp-clickhouse__run_select_query`

- Query Grafana for slow request patterns on the rooms endpoint
- Pull Loki logs filtered for slow query warnings
- Run `SHOW INDEX FROM GT_CLASS_ONLINE` and `SHOW INDEX FROM GT_CLASS` in the database
- Run `EXPLAIN` on the suspected queries to see if they're doing full table scans

#### Step 3: Specific Queries to EXPLAIN
```sql
-- The findByRoomId query
EXPLAIN SELECT * FROM GT_CLASS_ONLINE co
LEFT JOIN GT_CLASS c ON co.CLASS_ID = c.ID
LEFT JOIN GT_CLASS_COURSE cc ON c.CLASS_COURSE_ID = cc.ID
WHERE co.ROOM_ID = 'some-room-id';

-- The getActiveLectureList with timestamp function
EXPLAIN SELECT * FROM GT_CLASS
WHERE STUDENT_USER_ID = ?
AND TEACHER_USER_ID != 0
AND CITY = ?
AND CREDIT = 'REGIST'
AND (CLASS_STATE IS NULL OR CLASS_STATE NOT IN ('PREFINISH', 'FINISH'))
AND TIMESTAMP(CLASS_DATE, CLASS_END_TIME) > SYSDATE()
AND CLASS_TYPE = 'PODO';
```

#### Step 4: Monitoring Verification
- Check APM metrics for p50/p95/p99 latency on the endpoint
- Correlate the timeline of the index migration with the latency spike
- Check DB connection pool metrics for saturation

### Phase 4: Likely Root Causes (Ranked by Probability)

1. **HIGH: Index on `GT_CLASS_ONLINE.ROOM_ID` was dropped during migration**
   - The `findByRoomId` query would degrade from O(log n) to O(n) without this index
   - This is the most direct explanation for the reported symptoms

2. **HIGH: Missing composite index on `GT_CLASS` for common query patterns**
   - Queries filter by `(STUDENT_USER_ID, CREDIT, CLASS_STATE, CLASS_TYPE)` frequently
   - Without a composite index, every query does a partial or full table scan

3. **MEDIUM: `TIMESTAMP(CLASS_DATE, CLASS_END_TIME) > SYSDATE()` prevents index usage**
   - Function wrapping columns in WHERE clause forces full table scan
   - Should be refactored to use a pre-computed indexed column

4. **MEDIUM: N+1 sequential query pattern in room info endpoint**
   - 3 sequential DB calls amplifies any per-query slowness
   - Could be batched or JOINed into a single query

5. **LOW: Table growth crossed a threshold after migration timing coincidence**
   - GT_CLASS table grows with each lesson, may have hit critical size

### Phase 5: Recommended Fixes

```sql
-- Fix 1: Restore/add index on ROOM_ID
CREATE INDEX idx_class_online_room_id ON GT_CLASS_ONLINE (ROOM_ID);

-- Fix 2: Add composite index for common lecture queries
CREATE INDEX idx_class_student_status ON GT_CLASS (STUDENT_USER_ID, CREDIT, CLASS_TYPE, CLASS_STATE);

-- Fix 3: Add index for ticket-based queries
CREATE INDEX idx_class_ticket_id ON GT_CLASS (CLASS_TICKET_ID, CREDIT);

-- Fix 4: Add index on CLASS_ID for join performance
CREATE INDEX idx_class_online_class_id ON GT_CLASS_ONLINE (CLASS_ID);
```

**Code fix for timestamp function preventing index usage:**
```java
// Instead of: TIMESTAMP(CLASS_DATE, CLASS_END_TIME) > SYSDATE()
// Use: CLASS_DATE > CURRENT_DATE - 1 (pre-filter) + application-side filtering
// Or add a computed/virtual column with an index
```

---

## Structure I Followed

1. **Discovery** - Broad search for endpoint, entities, repositories
2. **Code Analysis** - Deep read of queries, entity definitions, controller flow
3. **History Check** - Git log for index migration evidence
4. **Pattern Identification** - Identified missing indexes, N+1 patterns, function-on-column anti-patterns
5. **Root Cause Ranking** - Prioritized by likelihood given the symptoms
6. **Fix Recommendations** - Concrete SQL and code changes

## Tools Used

| Tool | Purpose | Count |
|------|---------|-------|
| `Bash(ls)` | Directory structure exploration | 3 |
| `Glob` | File pattern matching (migrations, indexes) | 2 |
| `Grep` | Code pattern search (endpoints, entities, annotations, native queries) | 6 |
| `Read` | Deep file analysis (entities, repositories, controllers) | 4 |
| `Bash(git log)` | Recent commit history for migration evidence | 1 |

## Limitations of This Analysis

1. **No database access** - Could not run EXPLAIN or SHOW INDEX to confirm index state
2. **No monitoring data** - Could not verify latency metrics or slow query logs from Grafana/Loki
3. **No migration files found** - The project does not appear to use Flyway/Liquibase; schema changes may be applied externally
4. **Endpoint mismatch** - The reported `/api/v1/rooms` does not exist literally; closest is `/api/v1/lecture/getLeLectureRoomInfo` and `findByRoomId` in repositories
5. **Large native query file** - `LectureOnlineJpaRepository.java` is ~29K tokens; could not read it fully, only searched specific patterns
6. **No runtime profiling** - Without APM data, ranking is based on code analysis heuristics only

## How I Would Verify Findings

1. **Run EXPLAIN on suspected queries** against production-replica DB
2. **Check `SHOW INDEX FROM GT_CLASS_ONLINE`** to confirm ROOM_ID index existence
3. **Compare Grafana latency dashboards** before/after the migration date
4. **Add the missing indexes in staging** and measure improvement
5. **Load test the endpoint** with and without indexes to quantify the impact
