# Execution Trace: Payment Amount Mismatch Analysis (Without Skill)

## Request Summary

"결제 시스템에서 Apple IAP 영수증 금액과 DB에 저장된 결제 금액이 불일치하는 케이스가 발견됐어. 약 200건 정도 되고 모두 최근 1주일 이내에 발생했어. 환불 처리도 꼬여있을 수 있어. 분석해줘"

---

## Phase 1: Codebase Discovery

### Tools Used
- **Grep**: Pattern search for `payment`, `iap`, `receipt`, `apple`, `refund`, `환불` across `podo-backend/src`
- **Glob**: File listing for `**/payment/**/*.java`
- **Read**: Key files read in parallel

### Files Explored (in order)
1. `/Users/heechul/podo-backend/PAYMENT_V2_NOTES.md` -- Architecture overview (V1 SQS vs V2 direct)
2. `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/payment/gateway/PaymentGateway.java` -- Core payment orchestration (800+ lines, read in segments)
3. `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/modules/portone/service/PortoneV2Service.java` -- PortOne V2 SDK integration
4. `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/payment/dto/PaymentInfoDTO.java` -- DTOs for payment messages
5. `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/payment/exception/PaymentStatusException.java` -- Status validation exception
6. `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/payment/exception/AmountValidationException.java` -- Amount validation exception
7. `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/payment/validator/AmountValidator.java` -- Amount validation logic
8. `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/payment/validator/PaymentValidator.java` -- Validator interface
9. `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/modules/apple/service/AppleService.java` -- Apple Sign-In (NOT IAP)
10. `/Users/heechul/podo-backend/podo-docs/domains/payment/entities.md` -- Entity documentation
11. `/Users/heechul/podo-backend/podo-docs/database/tables/GT_PAYMENT_INFO.md` -- DB schema docs
12. `/Users/heechul/podo-backend/src/main/java/com/speaking/podo/applications/payment/controller/PaymentController.java` -- Webhook/API endpoints

### Key Grep Searches
- `apple|iap|in.app|appstore|storekit` in `podo-backend/src` -- Found 11 files, all related to Apple Sign-In auth, NOT Apple IAP
- `iap|in.app.purchase|storekit|receipt|RevenueCat` in `podo-app` -- Found matches in native app config and pnpm-lock but no dedicated IAP receipt validation module
- `AmountValidation|validateAmount` -- Found validation logic in PaymentGateway and AmountValidator
- `refund|환불` -- Found refund paths in PaymentGateway, TicketService, SubscribeMappService

---

## Phase 2: Architecture Understanding

### Payment Flow (as discovered)

```
[Client App] --(PortOne SDK)--> [PortOne PG]
                                      |
                                 (webhook)
                                      |
                                      v
[PaymentController /webhook] --> [PaymentGateway.processWebhook()]
                                      |
                                      v
                              [PortoneV2Service.getPayment()] -- queries PortOne for payment info
                                      |
                                      v
                              [PaymentValidator chain] (Order 1-3)
                                 1. PaymentStatusValidator
                                 2. UserValidator / CardValidator
                                 3. AmountValidator
                                      |
                                      v
                              [processPayment()] --> [addPayment()] --> DB insert
                                      |
                              +-------+--------+
                              |                |
                              v                v
                    processPaymentData   processPaymentSchedule
                    (ticket/subscribe)   (next billing schedule)
```

### Critical Finding: No Apple IAP Module Exists

The codebase has **no Apple IAP receipt validation or StoreKit server notification handling**. Key evidence:

1. `AppleService.java` is purely for Apple Sign-In (OAuth token generation/revocation)
2. No `receipt`, `storekit`, `iap`, `in-app-purchase` patterns found in payment domain code
3. All payment processing goes through **PortOne (formerly iamport)** as the single PG gateway
4. The `PortoneV2Service` handles all payment queries, schedule management, and cancellations via PortOne SDK
5. The app (`podo-app`) has no dedicated IAP receipt validation logic visible in the codebase

### Amount Validation Logic (AmountValidator.java)

The validation compares:
- `subscribeService.getSubPrice(subscribeDto)` -- the expected price from the Subscribe product table
- Minus coupon discount (FIXED or PERCENTAGE type)
- Against `portoneInfo.getAmount()` -- the actual charged amount from PortOne

```java
if (Stream.of(totalPrice, portoneInfo.getAmount()).distinct().count() != 1) {
    throw new AmountValidationException("결제 금액이 일치하지 않습니다.", ...);
}
```

### Refund Flow

When payment processing fails after charge:
1. `portoneV2Service.cancelPayment(paymentId, reason)` -- PortOne API cancel
2. `PaymentFailInfo` record saved to DB
3. Slack notification sent (`SLACK_PAYMENT_API_FAILED`)
4. Exception re-thrown for transaction rollback

DB refund representation:
- `PARENT_PAYMENT_ID` = original payment ID
- `PAID_AMOUNT` = negative value
- `REMAIN_AMOUNT` = post-refund balance

---

## Phase 3: Analysis of the Reported Issue

### Problem Statement Decomposition

| Aspect | Detail |
|--------|--------|
| What | Apple IAP receipt amount vs DB stored payment amount mismatch |
| Scale | ~200 cases |
| Timeframe | Last 7 days |
| Complication | Refund processing may also be corrupted |

### Root Cause Hypotheses

Given that **no Apple IAP integration exists in the codebase**, I would pursue these hypotheses in priority order:

#### Hypothesis 1: Misidentification -- These Are PortOne Payments, Not Apple IAP (HIGH probability)

The system processes ALL payments through PortOne. If the reporter used "Apple IAP" loosely, the actual issue could be:
- **Subscribe price changed** in the `LE_SUBSCRIBE` table (product price update) while scheduled billing payments used the old amount
- **Coupon discount calculation error** -- the `PERCENTAGE` discount path uses `Math.floor` and `discountAmountMax`, which could produce rounding mismatches
- **Currency conversion** -- PortOne returns amount in different precision than what's stored in `PAID_AMOUNT` (Integer)

**Investigation query:**
```sql
SELECT pi.ID, pi.USER_UID, pi.PAID_AMOUNT, pi.STATUS, pi.UPDATE_DATE,
       pi.SUBSCRIBE_MAPP_ID, pi.COUPON_NO, pi.MERCHANT_UID, pi.IMP_UID
FROM GT_PAYMENT_INFO pi
WHERE pi.UPDATE_DATE >= DATE_SUB(NOW(), INTERVAL 7 DAY)
  AND pi.STATUS = 'paid'
ORDER BY pi.UPDATE_DATE DESC;
```

Cross-reference with PortOne API:
```
For each record: PortoneV2Service.getPayment(merchantUid).getAmount().getTotal()
vs pi.PAID_AMOUNT
```

#### Hypothesis 2: V1-to-V2 Migration Data Inconsistency (MEDIUM probability)

The system is actively migrating from V1 (SQS-based) to V2 (direct webhook). Per `PAYMENT_V2_NOTES.md`:
- V1 uses `imp_uid` as payment identifier
- V2 uses `paymentId` (merchant-side order number)
- The `PaymentGateway` has both old `validateAmount()` (line 1337) and new `AmountValidator` component running

If both V1 and V2 paths are active, a payment could be:
1. Processed via V1 SQS with one amount interpretation
2. Stored in DB with a different amount due to the webhook handler using V2 `getPayment()` which returns `amount.total` (which may include tax differently)

**Investigation:**
```sql
-- Check if affected payments have V1-style IMP_UID vs V2-style MERCHANT_UID patterns
SELECT
  CASE WHEN IMP_UID LIKE 'imp_%' THEN 'V1' ELSE 'V2' END as version,
  COUNT(*)
FROM GT_PAYMENT_INFO
WHERE UPDATE_DATE >= DATE_SUB(NOW(), INTERVAL 7 DAY)
GROUP BY version;
```

#### Hypothesis 3: Race Condition in Concurrent Webhook Processing (MEDIUM probability)

The `PaymentGateway.processWebhook()` uses `LockManager` for distributed locking:
- Lock key: `payment:{impUid}`
- If lock acquisition fails: throws `ALREADY_PROCESSING` and skips (no refund)
- If lock acquired but processing fails: auto-refund + fail info saved

A race condition could cause:
1. First webhook: acquires lock, starts processing
2. Second webhook (retry from PortOne): fails lock, returns silently
3. First webhook fails mid-processing, auto-refunds
4. But DB record was already partially committed (due to `@Transactional` boundaries)

**Investigation:**
```sql
-- Check for duplicate IMP_UIDs in recent period
SELECT IMP_UID, COUNT(*) as cnt
FROM GT_PAYMENT_INFO
WHERE UPDATE_DATE >= DATE_SUB(NOW(), INTERVAL 7 DAY)
GROUP BY IMP_UID
HAVING cnt > 1;
```

#### Hypothesis 4: Scheduled Billing Amount Drift (MEDIUM probability)

For `BILLING` type (recurring payments), the amount is set at schedule creation time:
```java
var amount = new PaymentAmountInput(request.amount(), null, null);
```

If product prices change between schedule creation and execution:
- Schedule was created with old price
- PortOne charges the scheduled amount (old price)
- `AmountValidator` compares against current `subscribeService.getSubPrice()` (new price)
- Mismatch detected, but payment already charged

This would explain bulk mismatches appearing within a 1-week window (after a price change deployment).

**Investigation:**
```sql
-- Check Subscribe price history / recent changes
-- Look at le_payment_fail_info for AmountValidation failures
SELECT * FROM le_payment_fail_info
WHERE failed_at >= DATE_SUB(NOW(), INTERVAL 7 DAY)
  AND fail_reason LIKE '%금액%'
ORDER BY failed_at DESC;
```

### Refund Corruption Analysis

The refund path has potential issues:

1. **Double refund risk**: The `cancelPayment` in both `BaseException` and generic `Exception` catch blocks (lines 1058, 1098) could theoretically fire if the transaction boundary doesn't properly isolate. The `PortoneV2Service.cancelPayment()` does handle `PaymentAlreadyCancelledException`, but the DB-side `PaymentFailInfo` could be duplicated.

2. **Orphaned refund records**: If `portoneV2Service.cancelPayment()` succeeds but the subsequent `paymentService.addPaymentFailInfo()` fails, the PortOne side is refunded but DB has no record.

3. **Transaction rollback after refund**: The catch blocks call `cancelPayment()` (external API, not transactional) then `throw e` for rollback. If the transaction rolls back, the `PaymentFailInfo` insert is lost, but the PortOne cancellation is permanent. This creates a state where money is refunded but no DB record exists.

**Investigation query:**
```sql
-- Find payments refunded on PortOne but not in DB
-- Compare PortOne transaction list vs GT_PAYMENT_INFO where PAID_AMOUNT < 0
SELECT pi.ID, pi.IMP_UID, pi.MERCHANT_UID, pi.PAID_AMOUNT, pi.STATUS
FROM GT_PAYMENT_INFO pi
WHERE pi.UPDATE_DATE >= DATE_SUB(NOW(), INTERVAL 7 DAY)
  AND (pi.STATUS = 'cancelled' OR pi.PAID_AMOUNT < 0)
ORDER BY pi.UPDATE_DATE DESC;
```

---

## Phase 4: Recommended Investigation Steps

### Step 1: Data Collection (DB queries)

```sql
-- 1A. All payments in the last 7 days with mismatch indicators
SELECT
  pi.ID, pi.USER_UID, pi.IMP_UID, pi.MERCHANT_UID,
  pi.PAID_AMOUNT, pi.REMAIN_AMOUNT, pi.STATUS,
  pi.PAYMENT_DIV, pi.EVENT_TYPE, pi.COUPON_NO,
  pi.SUBSCRIBE_MAPP_ID, pi.UPDATE_DATE
FROM GT_PAYMENT_INFO pi
WHERE pi.UPDATE_DATE >= DATE_SUB(NOW(), INTERVAL 7 DAY)
ORDER BY pi.UPDATE_DATE DESC;

-- 1B. Failed payments in the period
SELECT * FROM le_payment_fail_info
WHERE failed_at >= DATE_SUB(NOW(), INTERVAL 7 DAY)
ORDER BY failed_at DESC;

-- 1C. Refund records
SELECT
  pi.ID, pi.PARENT_PAYMENT_ID, pi.PAID_AMOUNT, pi.REMAIN_AMOUNT,
  pi.STATUS, pi.UPDATE_DATE,
  parent.PAID_AMOUNT as original_amount
FROM GT_PAYMENT_INFO pi
LEFT JOIN GT_PAYMENT_INFO parent ON pi.PARENT_PAYMENT_ID = parent.ID
WHERE pi.UPDATE_DATE >= DATE_SUB(NOW(), INTERVAL 7 DAY)
  AND (pi.PAID_AMOUNT < 0 OR pi.STATUS = 'cancelled')
ORDER BY pi.UPDATE_DATE DESC;
```

### Step 2: PortOne Cross-Reference

For each of the ~200 mismatched records:
- Call `PortoneV2Service.getPayment(merchantUid)` to get PortOne's recorded amount
- Compare `portonePayment.getAmount().getTotal()` vs `GT_PAYMENT_INFO.PAID_AMOUNT`
- Check PortOne cancellation status vs DB status

### Step 3: Subscribe Price Audit

```sql
-- Check if any Subscribe product prices changed recently
-- (This requires access to a price change log or comparing current prices
--  against amounts stored in recent payment records)
SELECT DISTINCT pi.SUBSCRIBE_MAPP_ID, sm.SUBSCRIBE_ID, s.PRICE, pi.PAID_AMOUNT
FROM GT_PAYMENT_INFO pi
JOIN GT_SUBSCRIBE_MAPP sm ON pi.SUBSCRIBE_MAPP_ID = sm.ID
JOIN LE_SUBSCRIBE s ON sm.SUBSCRIBE_ID = s.ID
WHERE pi.UPDATE_DATE >= DATE_SUB(NOW(), INTERVAL 7 DAY)
  AND pi.PAID_AMOUNT != s.PRICE
  AND pi.COUPON_NO IS NULL;
```

### Step 4: Log Analysis

Search application logs for:
- `AmountValidationException` occurrences in the last 7 days
- `결제 금액이 일치하지 않습니다` error messages
- `WEBHOOK_PROCESS_FAIL` event types
- `결제 처리 중 오류 발생으로 인한 자동 환불` log entries

### Step 5: Grafana/Monitoring

- Check payment success/failure rate dashboard for the 7-day window
- Look for Slack alert channels receiving `SLACK_PAYMENT_API_FAILED` or `SLACK_PAYMENT_BILLING_FAIL`
- Check Sentry for clustered exceptions in payment flow

---

## Phase 5: Verification Plan

### How I Would Verify Findings

1. **Reproduce**: Take one specific mismatched payment ID, trace it through:
   - PortOne dashboard (actual charge amount)
   - DB record (stored amount)
   - Application logs (validation result)
   - Subscribe product table (expected price at time of charge)

2. **Pattern confirmation**: Group the 200 cases by:
   - PaymentType (BILLING vs FIRST_BILLING vs TRIAL)
   - Subscribe product ID (to check if one product is affected)
   - Time clustering (to correlate with a deployment or config change)
   - Coupon usage (to isolate coupon calculation bugs)

3. **Refund integrity check**:
   - For each of 200 cases, verify PortOne transaction status matches DB status
   - Identify any "phantom refunds" (PortOne refunded, DB shows paid)
   - Identify any "missed refunds" (DB shows cancelled, PortOne still charged)

---

## Structure Used (Organic, No Predefined Framework)

| Step | Activity | Time Spent | Tools |
|------|----------|-----------|-------|
| 1 | Initial codebase search for payment/IAP/refund keywords | ~2 min | Grep, Glob |
| 2 | Read architecture docs (PAYMENT_V2_NOTES.md) | ~1 min | Read |
| 3 | Read core payment files (PaymentGateway, PortoneV2Service, validators) | ~5 min | Read (parallel) |
| 4 | Search for Apple IAP integration (found: none exists) | ~1 min | Grep |
| 5 | Read DB schema docs and entity definitions | ~2 min | Read |
| 6 | Analyze refund flow in PaymentGateway catch blocks | ~2 min | Read |
| 7 | Formulate hypotheses based on code evidence | ~3 min | Analysis |
| 8 | Draft investigation queries and verification plan | ~3 min | Writing |

### Limitations of This Approach (Without Skill)

1. **No structured decomposition framework**: I jumped into code reading immediately rather than systematically decomposing the problem space first. A structured approach would first separate "what data do we need" from "where is the code."

2. **No parallel investigation tracks**: I explored files sequentially rather than running multiple analysis threads (e.g., simultaneously checking backend + app + infra).

3. **No severity/impact triage**: I did not formally categorize which hypotheses have the highest business impact or urgency before investigating.

4. **No explicit evidence chain**: My findings are based on code reading but lack formal traceability (e.g., "line X in file Y proves Z").

5. **No monitoring integration**: I did not query Grafana, ClickHouse, or Sentry for real-time data to correlate with code findings.

6. **No remediation plan**: I stopped at investigation steps without proposing concrete fixes or hotfix strategies.

7. **Single-pass analysis**: A production incident of this scale (200 cases, money involved) warrants iterative investigation with checkpoint verification, not a single-pass analysis.

---

## Key Conclusions

1. **There is no Apple IAP integration in this codebase.** All payments go through PortOne. The reporter likely means "Apple Pay via PortOne" or is conflating terminology. Clarification is needed.

2. **The most likely root cause is a Subscribe product price change** that created a mismatch between scheduled billing amounts and the current validation baseline in `AmountValidator`.

3. **The refund flow has a transactional integrity gap**: PortOne cancellation is a non-transactional external call inside a `@Transactional` method's catch block. If the DB transaction rolls back, the refund record is lost but the money is already returned to the customer. This could explain "refund processing corruption."

4. **Immediate actions needed**:
   - Run the investigation queries above against the production database
   - Pull PortOne transaction logs for the 200 affected payments
   - Check deployment history for the last 7 days for any price/config changes
   - Verify Slack alert channels for payment failure notifications that may have been missed
