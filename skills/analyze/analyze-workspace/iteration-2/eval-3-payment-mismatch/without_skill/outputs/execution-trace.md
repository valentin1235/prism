# Payment Mismatch Analysis Approach (Without Skill Guidance)

## Task
"결제 시스템에서 Apple IAP 영수증 금액과 DB에 저장된 결제 금액이 불일치하는 케이스가 발견됐어. 약 200건 정도 되고 모두 최근 1주일 이내에 발생했어. 환불 처리도 꼬여있을 수 있어. 분석해줘"

---

## 1. What Information Would I Gather

### 1.1 Codebase Exploration (Completed)
I explored the following areas across two repositories (`podo-backend`, `podo-app`):

- **Payment domain**: `PaymentGateway.java`, `PaymentInfo.java`, `PaymentDetail.java`, `PaymentServiceImpl.java`, `PaymentController.java`, `PaymentType.java`
- **Amount validation**: `AmountValidator.java`, `AmountValidationException.java`
- **Portone (PG integration)**: `PortoneV2Service.java`, `PortoneScheduleService.java`, `BillingKeyVersionDetector.java`
- **Apple service**: `AppleService.java` (only handles OAuth/Sign-in-with-Apple, NOT IAP)
- **Webhook processing**: `PortoneWebhookRequest.java`, `PaymentController.java` webhook endpoint
- **Subscribe/Ticket domain**: `SubscribeMapp`, `Ticket`, related services
- **Recent git history**: ~20 commits in the last 2 weeks touching payment/subscribe code

### 1.2 Critical Finding: No Apple IAP Implementation Exists
A thorough search for IAP/StoreKit/in-app-purchase/receipt-verification across both `podo-backend` and `podo-app` yielded **zero results**. The system uses **Portone (formerly iamport)** as its sole PG (Payment Gateway), processing payments via:
- Card billing (빌링키 기반 정기결제)
- Kakao Pay / Naver Pay (간편결제)
- V1-to-V2 migration in progress

The `AppleService.java` (`/Users/heechul/podo-backend/src/main/java/com/speaking/podo/modules/apple/service/AppleService.java`) only handles Apple Sign-In (OAuth JWT token generation and auth code exchange), not IAP receipt validation.

### 1.3 Data I Would Need (But Cannot Access from Code Alone)
- **Database queries**: The 200 mismatched records -- what are the `PAID_AMOUNT` values in `GT_PAYMENT_INFO` vs. what Portone reports?
- **Portone dashboard/logs**: Actual transaction amounts from the PG side
- **Server application logs**: Any `AmountValidationException` or `결제 금액이 일치하지 않습니다` log entries
- **Deployment timeline**: When was the latest code deployed relative to the 1-week window?
- **Environment specifics**: Production vs. staging, which Portone version (V1 vs V2)?

---

## 2. How I Would Structure the Investigation

### Phase 1: Clarify the Problem Statement
The task mentions "Apple IAP 영수증" but the codebase has NO Apple IAP implementation. This means either:
- **(A)** The reporter is using "Apple IAP" loosely to mean "payments made on iOS devices" (which go through Portone, not Apple IAP)
- **(B)** There is a separate microservice or external system handling Apple IAP that is not in this repository
- **(C)** Apple IAP is planned/partially implemented and the issue is about a missing integration

**Action**: Clarify with the reporter what "Apple IAP 영수증" specifically refers to. This is the single most important disambiguation.

### Phase 2: Analyze Amount Validation Path
If the issue is about Portone payment amount mismatches (hypothesis A), the validation path is:

1. **Webhook arrives** at `/api/v1/payment/podo/webhook` (`PaymentController.java:36-86`)
2. **`PaymentGateway.processWebhook()`** fetches payment info from Portone V2 API (`PaymentGateway.java:962`)
3. **`PaymentGateway.processPayment()`** runs validation (`PaymentGateway.java:189`)
4. **`AmountValidator.validate()`** compares subscribe price (with coupon discount) against Portone amount (`AmountValidator.java:39-71`)
5. On mismatch, `AmountValidationException` is thrown

Key code path at `AmountValidator.java:62-63`:
```java
if (Stream.of(totalPrice, portoneInfo.getAmount())
        .distinct().count() != 1) {
```

### Phase 3: Identify Potential Root Causes

**Hypothesis 1: V1-to-V2 Migration Amount Mapping Bug**
- The system is actively migrating from Portone V1 to V2 (evidenced by `PortoneV2Service.java` with V1 compat methods, `BillingKeyVersionDetector.java`, and `PortoneWebhookRequest.java` supporting both V1/V2 formats)
- `PortoneV2Service.paymentToMap()` at line 500: `map.put("amount", payment.getAmount() != null ? payment.getAmount().getTotal() : 0)` -- if `getTotal()` returns a different value than V1's `amount` field (e.g., including VAT differently), amounts would mismatch
- Recent commits show active V2 migration work

**Hypothesis 2: Coupon Discount Calculation Race Condition**
- `AmountValidator.java:49-59` applies coupon discounts to the subscribe price
- If a coupon's `discountAmount` or `discountAmountMax` changed between payment initiation and webhook processing, the server-side calculation would differ from the PG-side amount
- The `PERCENTAGE` discount path uses `Math.floor()` which could cause rounding differences

**Hypothesis 3: Subscribe Price Change During Billing Cycle**
- `AmountValidator.java:47`: `int totalPrice = subscribeService.getSubPrice(context.getSubscribeDto())`
- If subscribe prices were updated in the last week, scheduled billing payments would use the OLD price at PG level but the NEW price at validation time

**Hypothesis 4: Timezone/Date-Related Issues**
- Recent commits show multiple timezone fixes: `UTC→KST 변환 적용` (commit `51f1fc5e7`), `날짜 파싱 오류 수정` (commit `657c9a59f`), `end-of-day timestamp` fixes
- These date/time bugs could affect scheduled payment amounts if the wrong billing period is calculated

**Hypothesis 5: Integer Overflow / Type Mismatch**
- `PaymentInfo.paidAmount` is `Integer` (`PaymentInfo.java:43`)
- `PortoneV2Service.CreateScheduleRequest.amount` is `long` (line 792)
- `AmountValidator` compares `int totalPrice` with `portoneInfo.getAmount()` -- if `getAmount()` returns `Long` and the comparison uses `Stream.of()` with autoboxing, `Integer` vs `Long` would ALWAYS show as distinct even for equal values

### Phase 4: Investigate Refund Entanglement
For the refund concern:
- `PortoneV2Service.cancelPayment()` at lines 710-748 handles cancellation
- It supports both full and partial refunds
- `PaymentAlreadyCancelledException` is caught and wrapped as `PAYMENT_ALREADY_CANCELLED`
- The `remainAmount` field on `PaymentInfo.java:85` tracks remaining refundable amount
- If amount mismatches cause some payments to be recorded incorrectly, refund calculations based on `paidAmount` or `remainAmount` would also be wrong

---

## 3. Perspectives/Angles I Would Consider

### Technical Angles
1. **Data integrity**: Are the 200 records all the same direction (DB > PG or DB < PG)? Consistent direction points to systematic bug; random directions point to race conditions
2. **Payment type distribution**: Are they all BILLING (정기결제), or mixed? BILLING goes through scheduled payment which has a different amount-setting path
3. **V1 vs V2 distribution**: Were these processed through V1 or V2 webhook format? The migration could be the trigger
4. **Coupon involvement**: Do all 200 records involve coupons? If yes, the coupon calculation is the likely culprit

### Business Angles
5. **Financial exposure**: 200 records x average payment amount = total financial discrepancy. Need to categorize: overcharged (refund urgently) vs. undercharged (write-off decision)
6. **Regulatory risk**: Payment amount mismatches may violate payment processing regulations
7. **User impact**: Have any users complained? Are auto-refund mechanisms safe to run?

### Operational Angles
8. **Blast radius**: Are new transactions still being affected, or was this a one-time deployment issue?
9. **Rollback feasibility**: Can the recent changes (timezone fixes, V2 migration) be rolled back?

---

## 4. How I Would Verify Findings

### Step 1: Database Query
```sql
-- Find the 200 mismatched records
SELECT pi.id, pi.USER_UID, pi.PAID_AMOUNT, pi.IMP_UID, pi.MERCHANT_UID,
       pi.STATUS, pi.UPDATE_DATE, pi.SUBSCRIBE_MAPP_ID
FROM GT_PAYMENT_INFO pi
WHERE pi.UPDATE_DATE >= DATE_SUB(NOW(), INTERVAL 7 DAY)
  AND pi.STATUS = 'paid'
ORDER BY pi.UPDATE_DATE DESC;
```

### Step 2: Cross-Reference with Portone
For each `MERCHANT_UID`, call `PortoneV2Service.getPayment()` and compare `payment.getAmount().getTotal()` with `pi.PAID_AMOUNT`.

### Step 3: Check Application Logs
Search for:
- `결제 금액이 일치하지 않습니다` (AmountValidationException messages)
- `AmountValidationException` stack traces
- `V2 getPayment` errors
- Any `cancelPayment` calls that correspond to the affected records

### Step 4: Reproduce in Staging
- Create a test subscription with a known price
- Trigger a billing payment
- Compare the amount stored in DB vs. what Portone reports

### Step 5: Git Bisect
If the issue started exactly 1 week ago, use `git log --since="1 week ago"` to find the deployment that introduced the bug. The recent commits show heavy activity around:
- Timezone fixes (UTC->KST)
- Date parsing fixes
- Payment count handling

---

## 5. What My Output Would Look Like

### Summary
The codebase has **no Apple IAP implementation**. All payments flow through Portone (V1/V2). The "Apple IAP 영수증" terminology needs clarification -- the issue is likely about Portone payment amounts not matching DB records. The system is in active V1-to-V2 Portone migration, and recent commits (last 2 weeks) include multiple date/timezone fixes and payment logic changes that are strong candidates for the root cause.

### Most Likely Root Cause (Pending Data Verification)
**The V1-to-V2 Portone migration introduced an amount mapping discrepancy.** Specifically:
- `PortoneV2Service.paymentToMap()` at line 500 maps amount as `payment.getAmount().getTotal()`
- This `Total` amount in V2 may include/exclude VAT differently than V1
- The `AmountValidator` compares against `subscribeService.getSubPrice()` which was calibrated for V1 amounts

### Recommendations (Prioritized)

| Priority | Action | Effort | Impact |
|----------|--------|--------|--------|
| 1 | **Clarify "Apple IAP"** -- confirm whether this means iOS Portone payments or actual App Store IAP | Low | Critical -- determines entire investigation direction |
| 2 | **Query the 200 records** -- determine direction and magnitude of mismatch | Low | High -- narrows root cause |
| 3 | **Check V1 vs V2 distribution** -- are all 200 records processed via V2 webhook? | Low | High -- confirms migration hypothesis |
| 4 | **Audit `paymentToMap()` amount logic** -- verify V2 `getTotal()` matches V1 semantics | Medium | High -- likely fix location |
| 5 | **Freeze refund processing** for affected records until root cause confirmed | Low | High -- prevents cascading damage |
| 6 | **Add amount reconciliation logging** -- log both DB amount and PG amount at webhook processing time | Medium | Medium -- future detection |

### Trade-offs

| Option | Pros | Cons |
|--------|------|------|
| Rollback V2 migration | Immediately stops new mismatches | Loses V2 progress; V1 is deprecated |
| Hotfix amount mapping | Targeted fix, preserves V2 progress | Requires precise root cause identification first |
| Add reconciliation batch job | Catches future mismatches automatically | Does not fix existing 200 records |
| Manual correction of 200 records | Directly resolves user impact | Labor-intensive; error-prone without tooling |

### Key References
- `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/payment/validator/AmountValidator.java:62-63` -- amount comparison logic
- `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/modules/portone/service/PortoneV2Service.java:500` -- V2 amount mapping
- `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/payment/gateway/PaymentGateway.java:962` -- webhook fetches payment from Portone V2
- `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/payment/domain/PaymentInfo.java:43` -- `PAID_AMOUNT` Integer column
- `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/modules/apple/service/AppleService.java` -- Apple OAuth only, no IAP
- Recent commits: `51f1fc5e7` (UTC->KST fix), `657c9a59f` (date parsing fix), `ea0e9f9ab` (datetime deserializer)

---

## Execution Trace: Tools Used and Sequence

### Round 1: Broad Discovery (Parallel)
- `Glob` x6: Searched for `*payment*`, `*iap*`, `*receipt*`, `*purchase*`, `*refund*`, `*apple*` across `podo-backend`
- **Finding**: No IAP/receipt/purchase files. Apple files are only OAuth config.

### Round 2: Source Structure (Parallel)
- `Bash`: Listed `src/main/` directory structure (Java project)
- `Grep` x3: Searched `payment|Payment`, `apple|Apple|IAP`, `refund|Refund` across `src/`
- **Finding**: Payment domain under `applications/payment/`, PG integration under `modules/portone/`, Apple under `modules/apple/`

### Round 3: Domain File Listing (Parallel)
- `Glob` x4: Listed all `.java` files under `payment/`, `portone/`, `subscribe/`, `ticket/`
- **Finding**: ~40 payment files, ~14 portone files, ~50 subscribe files, ~17 ticket files

### Round 4: Core File Reading (Parallel)
- `Read` x6: `PaymentGateway.java` (first 200 lines), `PortoneV2Service.java`, `AppleService.java`, `PaymentInfo.java`, `PaymentDetail.java`, `AmountValidator.java`, `AmountValidationException.java`, `PaymentServiceImpl.java`
- **Finding**: Complete payment flow understood. AmountValidator is the key validation point.

### Round 5: Webhook and Controller (Parallel)
- `Read` x2: `PaymentGateway.java` (lines 900-1050), `PaymentController.java`
- `Grep` x2: Searched `webhook|Webhook` and `IAP|in.app|inApp|appstore` patterns
- **Finding**: Webhook flow fully traced. Zero IAP references confirmed.

### Round 6: App-Side and Git History (Parallel)
- `Grep`: Searched `IAP|StoreKit|apple.*pay` in `podo-app`
- `Bash`: `git log --since="2 weeks ago"` for payment-related files
- `Read`: `PaymentGateway.java` (lines 400-600), `PaymentType.java`
- **Finding**: No IAP in app code either. Recent commits show heavy date/timezone fixes.

### Total Tool Calls
- Glob: 10
- Grep: 8
- Read: 12
- Bash: 2
- **Total: 32 tool calls across 6 rounds**

### What I Did NOT Do (Gaps)
1. **Did not query the database** -- no DB access available from code analysis alone
2. **Did not check Portone V1 service** (`PortoneService.java`) to compare V1 vs V2 amount handling
3. **Did not read all PaymentGateway.java** (~1100+ lines, only read ~600 lines due to token limits)
4. **Did not check the `BillingKeyVersionDetector.java`** which determines V1/V2 routing
5. **Did not examine `PortoneScheduleService.java`** which handles scheduled billing amounts
6. **Did not search for any batch/cron jobs** that might reconcile or modify payment amounts
7. **Did not verify the hypothesis about Integer vs Long comparison** in AmountValidator

### Self-Assessment
- **Strengths**: Quickly identified that Apple IAP does not exist in the codebase (critical finding). Traced the full payment flow. Identified 5 plausible hypotheses with code references.
- **Weaknesses**: Without skill guidance, I explored broadly rather than deeply. The investigation lacked structured phases (e.g., no explicit hypothesis-test cycle). I did not systematically verify each hypothesis against the code. The output is more "here are possibilities" than "here is the root cause."
