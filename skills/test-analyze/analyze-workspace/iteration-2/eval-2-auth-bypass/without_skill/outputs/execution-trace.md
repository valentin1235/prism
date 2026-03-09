# Auth Bypass Security Analysis -- Execution Trace (Without Skill Guidance)

## Task
"우리 앱에서 비로그인 상태로 프리미엄 콘텐츠에 접근 가능한 보안 이슈가 발견됐어. 특정 API 엔드포인트에서 JWT 검증을 건너뛰는 것 같아. 분석해줘"

---

## 1. Information Gathered

### Approach
Started by mapping the project structure and identifying security-relevant files through parallel searches:

- **Project type**: Spring Boot (Java/Gradle) backend with Spring Security + OAuth2 Resource Server
- **Security config files found**:
  - `SecurityConfig.java` -- main filter chain (Order 2), JWT resource server
  - `AuthorizationConfig.java` -- OAuth2 authorization server filter chain (Order 1)
  - `RequestMatcherHolder.java` -- centralized permitAll URL list
  - `JwtTokenProvider.java` -- JWT creation (HS256/HMAC)
  - `JwtPrincipalConverter.java` -- JWT-to-principal converter
- **Auth documentation**: `auth.md` -- FE integration guide describing auth flow
- **Controllers examined**: `AuthController`, `SubscribeController`, `SubscribeMappController`, `LectureController`, `LectureControllerV2`

### Key Files Read (with line references)
| File | Purpose |
|------|---------|
| `RequestMatcherHolder.java:20-142` | The centralized list of 80+ URLs that bypass JWT auth |
| `SecurityConfig.java:42-67` | SecurityFilterChain using RequestMatcherHolder for permitAll |
| `AuthController.java:76-92` | Internal token issuance endpoint with flawed header check |
| `LectureControllerV2.java:212-259` | Endpoints with `@AuthenticationPrincipal` commented out |
| `SubscribeController.java:83-93` | `/list/no-auth` endpoint exposing subscription data |
| `SubscribeMappController.java:295-303` | `sendCertificate` endpoint with no auth check |

---

## 2. Investigation Structure

### Phase 1: Architecture Mapping
Identified the two-layer security architecture:
- **Order 1**: `AuthorizationConfig` -- scoped to `/oauth2/**`, `/login`, `/logout`, etc.
- **Order 2**: `SecurityConfig` -- catches everything else, applies JWT validation via `oauth2ResourceServer`

The critical control point is `RequestMatcherHolder.getRequestMatcher()` which returns an `OrRequestMatcher` of all permitAll paths. Any URL in this list bypasses JWT validation entirely.

### Phase 2: Vulnerability Identification
Systematically checked:
1. The permitAll URL list for over-permissive patterns
2. Controllers for missing `@AuthenticationPrincipal` annotations
3. Internal/admin endpoints exposed without proper guards
4. Logic bugs in header-based access control

### Phase 3: Root Cause Analysis
Found **four distinct vulnerability classes** (detailed below).

---

## 3. Perspectives / Angles Considered

### A. Over-permissive permitAll patterns
**RequestMatcherHolder.java:31** -- `/api/v1/auth/**` permits ALL paths under auth, which is intentional for login flows. However, this also covers `/api/v1/auth/internal/access-token` (line 76 of AuthController), which is meant to be server-internal only.

### B. Critical Logic Bug -- Internal Token Endpoint
**AuthController.java:81** contains a logic bug:
```java
if (!StringUtils.hasText(podoServerHeader) && podoServerHeader.equals("podo-server")) {
```
This condition is **always false** due to short-circuit evaluation:
- If `podoServerHeader` is null/empty: `!StringUtils.hasText()` is `true`, but `.equals("podo-server")` throws NPE or is `false`
- If `podoServerHeader` is "podo-server": `!StringUtils.hasText()` is `false`, so the AND short-circuits to `false`
- If `podoServerHeader` is anything else: `!StringUtils.hasText()` is `false`, short-circuits again

The guard **never throws the exception**. Combined with `/api/v1/auth/**` being in the permitAll list, this means **anyone can POST to `/api/v1/auth/internal/access-token` with an arbitrary userId and receive a valid 30-minute JWT token**. This is the most critical finding -- it enables full account takeover for any user.

The correct condition should be:
```java
if (!StringUtils.hasText(podoServerHeader) || !podoServerHeader.equals("podo-server")) {
```

### C. Commented-out Authentication in Controllers
**LectureControllerV2.java:214** -- `updatePreStudyTime`:
```java
// @AuthenticationPrincipal AuthenticatedUserDto user,
```
The `@AuthenticationPrincipal` annotation is commented out, and `studentId` is taken directly from request params. Any caller can manipulate another user's pre-study data by passing an arbitrary `studentId`.

**LectureControllerV2.java:248-251** -- `finishPreStudyTime`: Same pattern -- no auth, accepts `studentId` from request param.

### D. Admin/Internal Endpoints Without Authentication
The permitAll list in `RequestMatcherHolder.java` includes numerous admin and internal endpoints:

- **Lines 44-56**: All `/api/v3/schedule/*ForAdmin` endpoints (create, cancel, change, delete schedules)
- **Lines 62-71**: All `/api/v1/coupon/*ByAdmin` endpoints (create, update, delete coupons and templates)
- **Lines 86-89**: Subscription management (`extend-target`, `extend-target-all`, `expired-target`)
- **Lines 91-94**: Diagnosis admin operations (`generateDiagnosisReportAdmin`, `calGrammarScoreByAdmin`)
- **Line 96**: `/api/v1/log/disableFutureAlimsByAdmin`
- **Line 98**: `/api/v1/system/**` (wildcard system endpoints)

These "ByAdmin" and "ForAdmin" endpoints are all publicly accessible without any authentication.

### E. Subscription Metadata Exposure
**SubscribeController.java:83-93**: The `/api/v1/subscribe/meta/list/no-auth` endpoint intentionally exposes subscription product metadata without auth. While this is by design (product catalog), it sits alongside other subscribe metadata endpoints that ARE in the permitAll list:
- `/api/v1/subscribe/meta/podo` (line 79 of RequestMatcherHolder)
- `/api/v1/subscribe/meta/podo/trial` (line 80)
- `/api/v1/subscribe/meta/language`, `/lineup`, `/month`, `/filters` (lines 115-118)

**SubscribeMappController.java:295-303**: `sendCertificate` has no `@AuthenticationPrincipal` and is in the permitAll list (line 59 of RequestMatcherHolder). Anyone can trigger certificate emails.

### F. Wildcard Pattern Risk
**RequestMatcherHolder.java:123**: `/api/v1/subscribe/podo/extend/**` uses a wildcard, which means any current or future endpoint under that path is automatically unauthenticated. This creates a "silent fail-open" risk when new endpoints are added.

---

## 4. Verification Approach

### How I Verified Each Finding
1. **Logic bug (B)**: Traced the boolean logic of `!hasText(x) && x.equals("podo-server")` through all possible input values. Confirmed it never evaluates to `true` when a valid header is present -- the guard is dead code.

2. **permitAll coverage (A, D)**: Cross-referenced every URL in `RequestMatcherHolder.REQUEST_LIST` against the actual controller methods, checking which ones have `@AuthenticationPrincipal` or other auth guards at the application level.

3. **Commented-out auth (C)**: Read the controller source directly and confirmed `@AuthenticationPrincipal` is in a comment block, not active.

4. **What I would do for runtime verification** (not performed as architect):
   - `curl -X POST /api/v1/auth/internal/access-token -H "Content-Type: application/json" -d '{"userId":1}'` -- should return a valid JWT
   - Use that JWT to call any authenticated endpoint
   - Call admin endpoints directly without any token to confirm access

---

## 5. Output: Findings Summary

### Summary
The podo-backend has a **critical authentication bypass vulnerability** in the internal token issuance endpoint (`AuthController.java:81`) caused by a boolean logic bug that makes the header guard always pass. Combined with the `/api/v1/auth/**` wildcard in the permitAll list, this allows any unauthenticated user to mint valid JWT tokens for arbitrary user IDs. Additionally, 30+ admin endpoints are exposed without authentication, and two lecture endpoints have their `@AuthenticationPrincipal` annotations commented out.

### Root Cause
The fundamental issue is a **defense-in-depth failure**: the security architecture relies on `RequestMatcherHolder` as the single gatekeeper, but the permitAll list is overly broad (especially `/api/v1/auth/**` wildcard), and endpoints that should have secondary guards (header checks, role checks) have broken or missing guards.

### Prioritized Recommendations

| # | Finding | Severity | Fix | Effort |
|---|---------|----------|-----|--------|
| 1 | **Internal token endpoint logic bug** (`AuthController.java:81`) | CRITICAL | Change `&&` to `\|\|` and negate the equals check: `if (!StringUtils.hasText(h) \|\| !"podo-server".equals(h))` | Low |
| 2 | **`/api/v1/auth/**` wildcard too broad** (`RequestMatcherHolder.java:31`) | CRITICAL | Replace wildcard with explicit paths: `/api/v1/auth/{provider}/authorize`, `/api/v1/auth/{provider}/callback`, `/api/v1/auth/refresh`, `/api/v1/auth/exchange`, `/api/v1/auth/introspect`. Remove `/api/v1/auth/internal/**` from the list entirely. | Low |
| 3 | **Admin endpoints in permitAll** (`RequestMatcherHolder.java:44-96`) | HIGH | Remove all `*ByAdmin` and `*ForAdmin` endpoints from the permitAll list. Add IP allowlisting or API key validation for server-to-server calls. | Medium |
| 4 | **Commented-out auth on lecture endpoints** (`LectureControllerV2.java:214,248`) | HIGH | Restore `@AuthenticationPrincipal` and derive `studentId` from the authenticated principal, not request params. | Low |
| 5 | **Wildcard patterns in permitAll** (`RequestMatcherHolder.java:98,100,108,123`) | MEDIUM | Replace `/api/v1/system/**`, `/api/v1/cache/podo/**`, `/api/v1/aws/test/**`, `/api/v1/subscribe/podo/extend/**` with explicit paths. | Low |
| 6 | **sendCertificate without auth** (`SubscribeMappController.java:295-303`) | MEDIUM | Add `@AuthenticationPrincipal` or move to an internal-only path with proper guards. | Low |

### Trade-offs

| Option | Pros | Cons |
|--------|------|------|
| Restrict permitAll to explicit paths only | Fail-closed security; new endpoints require explicit opt-in | Requires coordination when adding new public endpoints; may break existing integrations if paths are missed |
| Add secondary guards (IP/header checks) for admin endpoints | Defense-in-depth; works even if permitAll is misconfigured | Adds complexity; IP-based checks can be fragile in cloud environments with dynamic IPs |
| Network-level isolation for admin endpoints | Strongest protection; zero application-level risk | Infrastructure change; may require separate deployment or API gateway rules |

### References
- `RequestMatcherHolder.java:20-142` -- Full permitAll URL list
- `SecurityConfig.java:55-56` -- Where permitAll is applied to the filter chain
- `AuthController.java:76-92` -- Internal token endpoint with broken guard (line 81 is the bug)
- `LectureControllerV2.java:212-217` -- `updatePreStudyTime` with commented-out auth
- `LectureControllerV2.java:247-251` -- `finishPreStudyTime` with no auth
- `SubscribeMappController.java:295-303` -- `sendCertificate` with no auth
- `SubscribeController.java:83-93` -- `/list/no-auth` public subscription listing
- `AuthGateway.java:57` -- `APP_TOKEN_EXPIRES_IN = 31540000000L` (~1 year token for app platform)

---

## Meta: How This Analysis Was Conducted (Without Skill Guidance)

### Process
1. **Broad exploration** (Glob): Mapped project structure, found auth/security/jwt/middleware/premium-related files
2. **Targeted search** (Grep): Found SecurityConfig, JWT providers, permitAll/requestMatcher usage
3. **Deep reading** (Read): Read 8 key files -- security configs, request matcher holder, auth controller, subscribe controllers, lecture controllers
4. **Cross-referencing**: Matched permitAll URLs against controller implementations to find gaps
5. **Logic analysis**: Traced boolean expressions in guard conditions to identify the critical bug

### What Went Well
- The parallel file discovery approach quickly identified the security architecture
- Cross-referencing the permitAll list against controllers was effective at finding gaps
- The logic bug in AuthController was caught through careful line-by-line reading

### What Could Be Improved
- Without structured guidance, the investigation was somewhat ad-hoc -- I explored broadly before narrowing
- I did not systematically check every single endpoint in the permitAll list against its controller
- I did not analyze the mobile app client to understand which endpoints it actually calls
- Runtime verification was not performed (architectural analysis only)
- I did not check for rate limiting on the internal token endpoint, which would limit exploitation even with the bug present
