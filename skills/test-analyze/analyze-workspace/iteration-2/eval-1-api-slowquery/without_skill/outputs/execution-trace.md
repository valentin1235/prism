# Execution Trace: API Slow Query Analysis (Without Skill Guidance)

## Task
"우리 서비스에서 API 응답이 평균 3초 이상 걸리고, DB 슬로우쿼리 로그가 대량 발생하고 있어. 특히 /api/v1/rooms 엔드포인트가 심각해. 최근 인덱스 관련 마이그레이션을 했는데 그 이후부터 발생한 것 같아. 분석해줘"

---

## 1. Information Gathering Approach

### What I actually did (codebase exploration):

1. **Project structure discovery** -- Identified this as a Java/Spring Boot + Gradle project (`podo-backend`) with JPA/QueryDSL, targeting MySQL.

2. **Endpoint search** -- Searched for `/api/v1/rooms` across the entire codebase. **Result: No such endpoint exists.** The actual API controllers use:
   - `/api/v1/lecture` (LectureController)
   - `/api/v2/lecture` (LectureControllerV2)
   - The "rooms" concept exists only as external API calls to Pagecall (`PagecallService`) and Lemonboard (`LemonBoardAdapterImpl`), not as an internal endpoint.

3. **Migration file review** -- Read all 3 SQL migration files:
   - `nps_response_ddl.sql` -- Creates `le_nps_response` table with indexes on `student_id`, `tutor_id`, `class_id`
   - `tutor_exclusion_ddl.sql` -- Creates `le_tutor_exclusion` and `le_tutor_exclusion_history` tables
   - `tutor_exclusion_add_class_id.sql` -- Adds `class_id` column to both tutor exclusion tables

4. **Git history analysis** -- Checked recent 30 commits and searched for index/migration related commits. Found reference to `PODOZ-1176/migrate-to-lemonboard` as a major migration effort.

5. **Native query inventory** -- Scanned all `@Query(nativeQuery=true)` annotations in the lecture repository layer. Found extensive native SQL queries in `LectureOnlineJpaRepository` and `LecturePrestudyLogRepository`.

6. **Entity/Table mapping** -- Identified key entities: `LectureOnline` (GT_CLASS_ONLINE table), `Lecture` (GT_CLASS), `LectureCourse` (GT_CLASS_COURSE), and their relationships.

7. **QueryDSL repository review** -- Read `LectureDslRepositoryImpl` which contains complex queries with joins, subqueries, and group-by operations on lecture-related tables.

### What I would additionally gather in a production scenario:

- **DB slow query logs**: Actual SQL statements with execution times, filtered to the timeframe after the migration
- **EXPLAIN/EXPLAIN ANALYZE output**: For the identified slow queries to see if indexes are being used
- **APM traces**: Request-level timing breakdown (controller -> service -> repository -> DB)
- **MySQL `SHOW INDEX` output**: For the affected tables to verify index state post-migration
- **Connection pool metrics**: To rule out pool exhaustion as a contributing factor
- **Table statistics**: Row counts, data distribution for affected tables

---

## 2. Investigation Structure

### Phase 1: Reproduce and Scope
- Confirm which exact endpoint(s) are affected (the stated `/api/v1/rooms` does not exist -- clarify with the reporter whether they mean `/api/v1/lecture/getLeLectureRoomInfo` or another endpoint)
- Establish baseline: what was the response time before the migration?
- Identify the exact migration that was applied and its timestamp

### Phase 2: Root Cause Analysis
- **Migration delta**: Compare the DB schema before and after the migration. Specifically check:
  - Were any existing indexes dropped (intentionally or accidentally)?
  - Were new columns added without corresponding indexes?
  - Did an ALTER TABLE cause an index rebuild or table lock?
- **Query plan analysis**: Run EXPLAIN on the slow queries to check:
  - Full table scans (type=ALL)
  - Temporary table usage (Using temporary)
  - Filesort operations (Using filesort)
  - Index usage (possible_keys vs key)

### Phase 3: Pattern Analysis
- Identify which query patterns are most affected
- Check if the N+1 query problem exists (e.g., `getLeLectureRoomInfo` at `LectureController.java:182` calls 3 separate queries: room info, audio list, video list)
- Look for missing pagination on list endpoints
- Check for inefficient joins in native queries

### Phase 4: Hypothesis and Fix
- Formulate specific index recommendations
- Validate with EXPLAIN that the proposed index would be used
- Test in staging environment

---

## 3. Perspectives and Angles Considered

### A. Database Layer
- **Index health**: Did the migration ALTER TABLE operations invalidate or lock existing indexes? MySQL's `ALTER TABLE ... ADD COLUMN` on InnoDB does an online DDL but may require index rebuilds depending on the operation.
- **Query optimization**: The native queries in `LectureOnlineJpaRepository` contain multi-table JOINs (GT_CLASS, GT_CLASS_ONLINE, GT_CLASS_COURSE, etc.) that could degrade without proper indexes.
- **Statistics staleness**: After DDL changes, MySQL optimizer statistics may be stale. `ANALYZE TABLE` may be needed.

### B. Application Layer
- **N+1 queries**: `getLeLectureRoomInfo` (LectureController.java:182-232) makes 3 sequential DB calls (room info, audio list, video list), then does in-memory stream processing for Kollus URL generation.
- **External API calls**: The Lemonboard and Pagecall adapters make synchronous HTTP calls. If these are in the critical path, they add latency independent of DB issues.
- **No pagination**: Many list endpoints (`getReserveLemonadeLectureList`, `getFinishLemonadeLectureList`, `getPodoClassCourseList`) return unbounded result sets.

### C. Infrastructure Layer
- **Connection pool saturation**: Slow queries hold connections longer, causing cascading delays for other requests.
- **DB server load**: Multiple slow queries competing for the same table locks or buffer pool space.

### D. Migration-Specific Risks
- The `tutor_exclusion_add_class_id.sql` migration adds a column but does NOT add an index on `class_id` for either table. If queries now filter by `class_id` on these tables, they would do full table scans.
- The NPS response table migration creates new indexes that may not yet have optimized statistics.

---

## 4. Verification Methods

### Immediate verification:
1. **Query the slow query log**: `SELECT * FROM mysql.slow_log WHERE start_time > '[migration_timestamp]' ORDER BY query_time DESC LIMIT 50;`
2. **Check index state**: `SHOW INDEX FROM GT_CLASS; SHOW INDEX FROM GT_CLASS_ONLINE; SHOW INDEX FROM le_tutor_exclusion;`
3. **Run EXPLAIN**: On each identified slow query with representative parameter values
4. **Check table sizes**: `SELECT table_name, table_rows, data_length, index_length FROM information_schema.tables WHERE table_schema = 'podo';`

### Post-fix verification:
1. Re-run EXPLAIN to confirm index usage
2. Monitor slow query log for 24h after fix
3. Check APM response time percentiles (p50, p95, p99) return to pre-migration baseline
4. Run load test against the affected endpoint(s)

---

## 5. Expected Output Structure

## Summary
The reported `/api/v1/rooms` endpoint does not exist in the codebase. The closest match is `/api/v1/lecture/getLeLectureRoomInfo` (LectureController.java:182) which queries lecture room data. Without access to production DB slow query logs and EXPLAIN output, the analysis is limited to code-level findings.

## Analysis

### Finding 1: Endpoint Mismatch
- **File**: `src/main/java/com/speaking/podo/applications/lecture/controller/LectureController.java:29`
- The controller maps to `/api/v1/lecture`, not `/api/v1/rooms`. Need clarification from the reporter.

### Finding 2: N+1 Sequential Query Pattern in Room Info
- **File**: `LectureController.java:182-232`
- `getLeLectureRoomInfo` makes 3 sequential DB calls plus in-memory Kollus URL generation. This is inherently slow even with good indexes.

### Finding 3: Missing Index on New Column
- **File**: `src/main/resources/db/migration/tutor_exclusion_add_class_id.sql:2`
- The migration adds `class_id` to `le_tutor_exclusion` without creating an index. If any query filters on this column, it will cause a full table scan.

### Finding 4: Unbounded Result Sets
- **Files**: Multiple endpoints in `LectureController.java` (lines 137, 160, 258, 288, 376)
- No pagination on list queries. As data grows, these become progressively slower.

### Finding 5: Complex Native Queries Without Index Hints
- **File**: `LectureOnlineJpaRepository.java` (multiple native queries)
- Multi-table JOINs on GT_CLASS, GT_CLASS_ONLINE, GT_CLASS_COURSE. After schema changes, the query planner may choose suboptimal plans.

## Root Cause (Hypothesis)
The most likely root cause is that the recent migration changed table structure (adding columns) which either:
1. Invalidated the query optimizer's cached statistics, causing suboptimal query plans
2. Added columns without corresponding indexes that are now used in WHERE/JOIN clauses
3. Triggered an InnoDB table rebuild that temporarily degraded performance

## Recommendations
1. **[HIGH PRIORITY, LOW EFFORT]** Run `ANALYZE TABLE` on all affected tables to refresh optimizer statistics
2. **[HIGH PRIORITY, LOW EFFORT]** Add index on `le_tutor_exclusion.class_id` if queries filter by it
3. **[MEDIUM PRIORITY, MEDIUM EFFORT]** Add pagination to all list endpoints (LectureController.java lines 137-178, 258-285, 376-395)
4. **[MEDIUM PRIORITY, LOW EFFORT]** Consolidate the 3 sequential queries in `getLeLectureRoomInfo` into a single JOIN query
5. **[LOW PRIORITY, HIGH EFFORT]** Add query-level monitoring/logging to identify the exact slow queries without relying on DB-level slow query logs

## Trade-offs

| Option | Pros | Cons |
|--------|------|------|
| ANALYZE TABLE | Immediate, zero-risk fix | May not be the root cause; temporary relief |
| Add missing indexes | Directly addresses full table scans | Additional write overhead; needs testing |
| Add pagination | Bounds worst-case query time | Breaking API change for clients |
| Query consolidation | Reduces round trips by ~66% | Increases query complexity; harder to maintain |

## References
- `LectureController.java:29` -- `/api/v1/lecture` base mapping (no `/api/v1/rooms` exists)
- `LectureController.java:182-232` -- `getLeLectureRoomInfo` with 3 sequential queries
- `tutor_exclusion_add_class_id.sql:2` -- ALTER TABLE without index
- `LectureDslRepositoryImpl.java:44-61` -- QueryDSL queries filtering on status, studentId, classState
- `LectureOnline.java:9` -- GT_CLASS_ONLINE entity with no declared indexes

---

## Meta: Approach Evaluation

### What worked well:
- Parallel codebase exploration (Glob + Grep + Read simultaneously) to quickly map the project
- Identifying the endpoint mismatch early (the reported endpoint doesn't exist)
- Tracing the full request path: controller -> service -> repository -> entity -> table

### What was limited without skill guidance:
- **No structured framework**: Analysis was ad-hoc; jumped between code exploration without a systematic checklist
- **No multi-perspective decomposition**: Did not systematically separate DB, application, infrastructure, and external dependency layers from the start
- **No severity/impact scoring**: Findings were not ranked by business impact
- **No timeline correlation**: Did not formally correlate git commit timestamps with the reported onset of slowness
- **Single-pass analysis**: Explored the code once rather than iterating with hypotheses
- **No explicit "what I don't know" section**: Gaps in knowledge (production metrics, actual slow query logs, DB schema state) were noted but not formally tracked
- **No recommended monitoring additions**: Did not suggest observability improvements for future incidents
