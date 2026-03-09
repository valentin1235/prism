# Execution Trace: Auth Bypass Security Analysis (Without Skill)

## Analysis Request
"우리 앱에서 비로그인 상태로 프리미엄 콘텐츠에 접근 가능한 보안 이슈가 발견됐어. 특정 API 엔드포인트에서 JWT 검증을 건너뛰는 것 같아. 분석해줘"

## Approach Taken (Baseline - No Predefined Skill)

### Phase 1: Codebase Discovery
**Tools used:** `Bash(ls)`, `Grep`, `Glob`

1. Listed project roots (`/Users/heechul/podo-backend`, `/Users/heechul/podo-app`)
2. Searched for JWT/auth-related files across the backend `src/` directory
3. Searched for `permitAll`/`anonymous`/`ignoring` patterns to find auth bypass configs
4. Searched for "premium" keyword (found none -- the app uses "subscribe"/"ticket"/"lecture" domain model)

**Time cost:** Initial broad search required iterating on domain terminology since "premium" doesn't exist in the codebase. Had to pivot to `subscription|content|lesson|lecture|course` patterns.

### Phase 2: Security Configuration Deep Dive
**Tools used:** `Read`

Read the core security files:
- `SecurityConfig.java` -- Spring Security filter chain with OAuth2 Resource Server JWT validation
- `AuthorizationConfig.java` -- OAuth2 Authorization Server (Order 1)
- `RequestMatcherHolder.java` -- **Central permitAll URL registry** (the key file)
- `JwtTokenProvider.java` -- HS256 JWT creation with Nimbus JOSE
- `JwtPrincipalConverter.java` -- Converts JWT subject to UserDetails

### Phase 3: Vulnerability Identification
**Tools used:** `Read`, `Grep`

Read controllers mapped to the permitAll endpoints to determine what data they expose and whether they have secondary auth checks.

---

## Findings

### CRITICAL: Arbitrary JWT Token Issuance via `/api/v1/auth/internal/access-token`

**Severity: CRITICAL**
**File:** `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/auth/controller/AuthController.java` (line 76-92)

The `/api/v1/auth/**` wildcard in `RequestMatcherHolder` (line 31) grants permitAll access to ALL auth endpoints, including `/api/v1/auth/internal/access-token`.

This internal endpoint issues a valid 30-minute JWT + refresh token for **any arbitrary userId**. The "protection" is a header check:

```java
if (!StringUtils.hasText(podoServerHeader) && podoServerHeader.equals("podo-server")) {
    throw new BaseException(UNAUTHORIZED, "올바른 헤더가 아닙니다.");
}
```

**This guard has a logic bug:** `!StringUtils.hasText(podoServerHeader)` returns `true` when the header is missing/blank, but then `podoServerHeader.equals("podo-server")` would be `false` (or NPE if null). The `&&` means the exception is thrown **only when the header is both empty AND equals "podo-server"** -- which is impossible. Therefore:
- If header is missing/null: `!hasText(null)` = `true`, `null.equals(...)` = **NullPointerException** (likely caught as 500, not a proper auth check)
- If header is present with any value (including wrong value): `!hasText("wrong")` = `false`, so the AND short-circuits and **the guard never throws** -- any non-empty header value bypasses the check
- If header is "podo-server" (correct value): `!hasText("podo-server")` = `false`, short-circuits, guard passes

**Impact:** An attacker can call `POST /api/v1/auth/internal/access-token` with `X-Podo-Header: anything` and `{"userId": 1}` to obtain a valid JWT for any user. This is a full authentication bypass that grants access to all authenticated endpoints.

### HIGH: Admin/Scheduler Endpoints Exposed Without Authentication

**Severity: HIGH**
**File:** `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/modules/web/security/RequestMatcherHolder.java`

The following admin/operational endpoints are in the permitAll list with **no secondary authentication**:

| Endpoint Pattern | Risk |
|---|---|
| `/api/v3/schedule/createBaseSchedule` | Can create tutor base schedules |
| `/api/v3/schedule/createScheduleByUnit` | Can create unit schedules |
| `/api/v3/schedule/createScheduleByBase` | Can create schedules by base |
| `/api/v3/schedule/changeByAdmin` | Can modify schedules |
| `/api/v3/schedule/cancelByAdmin` | Can cancel schedules |
| `/api/v3/schedule/deleteByUnits` | Can delete schedule units |
| `/api/v3/schedule/deleteByRange` | Can delete schedule ranges |
| `/api/v3/schedule/cancelAndReassignForAdmin` | Can cancel and reassign |
| `/api/v1/coupon/createCouponTemplateByAdmin` | Can create coupon templates |
| `/api/v1/coupon/publishCouponByAdmin` | Can publish coupons |
| `/api/v1/coupon/deleteCouponByAdmin` | Can delete coupons |
| `/api/v1/subscribe/podo/extend/**` | Can extend subscriptions |
| `/api/v1/system/**` | Full system endpoint access |
| `/api/v1/cache/podo/**` | Can manipulate cache |
| `/api/v1/diagnosis/generateDiagnosisReportAdmin` | Can generate diagnosis reports |

These appear to be internal/admin endpoints that were added to permitAll presumably because they are called by internal services or admin tools, but they are fully accessible from the public internet without any authentication.

### HIGH: AI Chat Endpoint Exposed Without Authentication

**Severity: HIGH**
**File:** `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/ai/controller/AiController.java`

Endpoints `/api/v1/ai/chat` and `/api/v1/ai/models` are in the permitAll list. The `/api/v1/ai/chat` endpoint takes arbitrary `ChatRequest` bodies and forwards them to an AI service. This allows unauthenticated users to consume AI API credits.

### MEDIUM: Lecture Endpoints with Commented-Out Auth

**Severity: MEDIUM**
**File:** `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/lecture/controller/LectureControllerV2.java` (line 212-258)

The `updatePreStudyTime` endpoint (line 212) has `@AuthenticationPrincipal` commented out (`// @AuthenticationPrincipal AuthenticatedUserDto user`) and accepts `studentId` as a query parameter. While this specific endpoint is not in the permitAll list (so JWT is still required), it means that **any authenticated user can update pre-study time for any other student** by passing a different `studentId`. Combined with the CRITICAL finding above, this becomes exploitable.

Similarly, `changePagecallToLemonboard` (line 345) and `finishPreStudyTime` (line 247) accept raw `studentId` parameters without verifying ownership, and both endpoints ARE in the permitAll list.

### MEDIUM: Subscription Data Exposure

**Severity: MEDIUM**

Several subscription endpoints in the permitAll list expose business data:
- `/api/v1/subscribe/podo/extend-target` -- Returns list of all extend targets
- `/api/v1/subscribe/podo/extend-target-all` -- Returns ALL extend target data
- `/api/v1/subscribe/podo/expired-target/{day}` -- Returns expired subscription targets
- `/api/v1/subscribe/meta/list/no-auth` -- Intentionally public (listing subscribe options)

### LOW: Swagger UI Exposed

**Severity: LOW**

`/swagger/**`, `/swagger-ui/**`, `/v3/api-docs/**` are in the permitAll list, which exposes the full API documentation including all endpoint signatures, request/response schemas, and internal naming conventions.

---

## Structure Followed

Without a predefined skill, I followed this ad-hoc structure:

1. **Orientation** -- Find the project structure, identify backend as Spring Boot (Kotlin/Java)
2. **Security config hunting** -- Search for Spring Security configuration, JWT handling, filter chains
3. **PermitAll surface mapping** -- Identify all endpoints that bypass JWT verification
4. **Controller cross-reference** -- For each permitAll endpoint, read the controller to check:
   - Does it have secondary auth checks (`@AuthenticationPrincipal` null guard)?
   - Does it expose sensitive data?
   - Does it perform state-changing operations?
5. **Attack chain construction** -- Map out how findings chain together

## Tools Used Summary

| Tool | Count | Purpose |
|---|---|---|
| `Bash(ls)` | 2 | Directory listing for orientation |
| `Grep` | ~10 | Pattern search for auth annotations, endpoint mappings, bypass patterns |
| `Read` | ~12 | Deep reading of security config, controllers, token services |
| `Glob` | 1 | Find AI controller files |

## Verification Approach

To verify these findings, I would:

1. **CRITICAL (Internal Token):** `curl -X POST http://host/api/v1/auth/internal/access-token -H "X-Podo-Header: x" -H "Content-Type: application/json" -d '{"userId":1}'` -- Should return a valid JWT
2. **HIGH (Admin endpoints):** `curl http://host/api/v3/schedule/getOpenSchedulesForAdmin` -- Should return schedule data without any token
3. **HIGH (AI endpoint):** `curl -X POST http://host/api/v1/ai/chat -H "Content-Type: application/json" -d '{"message":"test"}'` -- Should process AI request
4. **MEDIUM (Lecture):** Call `finishPreStudyTime` or `updatePreStudyTime` with any studentId -- should succeed without auth

## What I Would Do Differently With a Predefined Workflow

Without a skill, this analysis was:
- **Ad-hoc in structure** -- I chose what to investigate based on intuition rather than a systematic checklist
- **Potentially incomplete** -- I focused on the backend Spring Security layer but did not:
  - Check if there's an API gateway (nginx, Kong, etc.) that might add a layer of protection
  - Examine the frontend app for client-side auth bypasses
  - Check for rate limiting on the exposed endpoints
  - Look for IP whitelisting configuration
  - Analyze the refresh token flow for vulnerabilities
  - Check for IDOR across all authenticated endpoints (not just the ones I noticed)
- **Unstructured output** -- The findings are organized by severity but the analysis path was non-linear
- **No automated verification** -- All verification steps are proposed but not executed
- **Single-pass** -- No iterative review or cross-validation of findings

A predefined analysis skill would provide:
1. A systematic checklist ensuring no auth surface is missed
2. Structured output format for consistent reporting
3. Automated verification steps
4. Multi-pass review to catch issues the first pass missed
5. Severity scoring framework aligned with OWASP or CVSS
6. Remediation recommendations template
