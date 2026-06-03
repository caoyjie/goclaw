# Ark Middleware Reliability Improvements — Test Plan

**Date:** 2026-05-05  
**Feature:** Startup sweep for stale indexing states + exponential backoff retry for VikingDB calls  
**Branch:** `ceo_ip`  
**Status:** Implementation complete, ready for testing

---

## Overview

This test plan covers the reliability improvements implemented in the ark-middleware service:

1. **Startup sweep** — Automatically resets rows stuck in `indexing`/`deindexing` states on service startup
2. **Exponential backoff retry** — Wraps VikingDB `add_doc`/`delete_doc` calls with 3-attempt retry (1s, 2s, 4s delays)

**Goal:** Prevent rows from getting permanently stuck after process crashes and improve resilience against transient VikingDB failures.

---

## Implementation Summary

### Components Implemented

| Component | File | Purpose |
|-----------|------|---------|
| Retry helper | `core/retry.py` | Generic async retry with exponential backoff |
| VikingDB client | `core/client.py` | Wraps VikingDB SDK calls with retry logic |
| Sweep functions | `core/db.py` | Queries and resets stale rows in PostgreSQL |
| FastAPI lifespan | `main.py` | Calls sweep functions on startup |
| Configuration | `core/config.py` | `STALE_MINUTES` setting (default 15) |

### Test Coverage

**Unit tests:** 19 tests, all passing
- Retry logic: 5 tests
- VikingDB client retry: 4 tests
- Sweep functions: 6 tests
- Lifespan integration: 4 tests

---

## Test Scenarios

### 1. Startup Sweep — Happy Path

**Objective:** Verify sweep functions correctly reset stale rows on startup.

**Prerequisites:**
- PostgreSQL database with `ark_files` table
- Service configured with valid `DATABASE_URL`
- `STALE_MINUTES=15` (default)

**Test Steps:**

1. **Setup:** Insert test rows into `ark_files`:
   ```sql
   INSERT INTO ark_files (id, status, updated_at) VALUES
     ('test-1', 'indexing', now() - interval '20 minutes'),  -- stale
     ('test-2', 'indexing', now() - interval '5 minutes'),   -- fresh
     ('test-3', 'deindexing', now() - interval '30 minutes'), -- stale
     ('test-4', 'indexed', now() - interval '1 hour');       -- not affected
   ```

2. **Action:** Start the ark service
   ```bash
   cd services/ark
   python -m uvicorn main:app --host 0.0.0.0 --port 8000
   ```

3. **Expected Results:**
   - Service starts successfully
   - Logs show: `"Startup sweep recovered 2 stale indexing + 1 stale deindexing rows"` (WARNING level)
   - Database state after startup:
     ```sql
     SELECT id, status, error_msg FROM ark_files WHERE id LIKE 'test-%';
     ```
     | id | status | error_msg |
     |----|--------|-----------|
     | test-1 | stored | Reset by startup sweep: process restarted while indexing |
     | test-2 | indexing | NULL |
     | test-3 | indexed | Reset by startup sweep: process restarted while deindexing |
     | test-4 | indexed | NULL |

**Pass Criteria:**
- ✅ Service starts without errors
- ✅ Stale rows (>15 min old) are reset to `stored`/`indexed`
- ✅ Fresh rows (<15 min old) are unchanged
- ✅ Warning log appears with correct counts
- ✅ `error_msg` contains explanatory text

---

### 2. Startup Sweep — No Stale Rows

**Objective:** Verify sweep handles clean database gracefully.

**Test Steps:**

1. **Setup:** Ensure no stale rows exist (all `indexing`/`deindexing` rows are recent)

2. **Action:** Start the ark service

3. **Expected Results:**
   - Service starts successfully
   - No warning log (sweep returns 0 for both functions)
   - Database unchanged

**Pass Criteria:**
- ✅ Service starts without errors
- ✅ No warning log about recovered rows
- ✅ No database modifications

---

### 3. Startup Sweep — Database Error Handling

**Objective:** Verify service starts even if sweep fails.

**Test Steps:**

1. **Setup:** Configure invalid `DATABASE_URL` or stop PostgreSQL

2. **Action:** Start the ark service

3. **Expected Results:**
   - Service logs error: `"Startup sweep failed for indexing: <error details>"` (ERROR level)
   - Service logs error: `"Startup sweep failed for deindexing: <error details>"` (ERROR level)
   - Service continues startup (doesn't crash)

**Pass Criteria:**
- ✅ Service doesn't crash on sweep failure
- ✅ Errors are logged with details
- ✅ Sweep functions return 0 on error

---

### 4. VikingDB Retry — Transient Failure Recovery

**Objective:** Verify retry logic recovers from transient VikingDB failures.

**Prerequisites:**
- VikingDB collection configured
- TOS bucket accessible

**Test Steps:**

1. **Setup:** Simulate transient failure (e.g., network blip, rate limit)
   - Option A: Use network proxy to inject 1-2 failures before success
   - Option B: Mock VikingDB SDK in test environment

2. **Action:** Call `add_doc` or `delete_doc` via API

3. **Expected Results:**
   - First attempt fails (logged as WARNING)
   - Second attempt succeeds
   - Total time: ~1 second (1s delay after first failure)
   - API returns success

**Pass Criteria:**
- ✅ Operation succeeds after transient failure
- ✅ Retry delays are exponential (1s, 2s, 4s)
- ✅ Warning logs show attempt count and delay
- ✅ Final result is correct

---

### 5. VikingDB Retry — Permanent Failure

**Objective:** Verify retry exhaustion after 3 attempts.

**Test Steps:**

1. **Setup:** Simulate permanent failure (e.g., invalid API key, missing collection)

2. **Action:** Call `add_doc` or `delete_doc` via API

3. **Expected Results:**
   - 3 attempts made (logged as WARNING for attempts 1-2)
   - Total time: ~7 seconds (1s + 2s + 4s delays)
   - Final exception raised to caller
   - API returns 500 error

**Pass Criteria:**
- ✅ Exactly 3 attempts made
- ✅ Exponential backoff applied (1s, 2s, 4s)
- ✅ Final exception propagates to caller
- ✅ Error response includes details

---

### 6. Configuration — Custom STALE_MINUTES

**Objective:** Verify `STALE_MINUTES` setting is respected.

**Test Steps:**

1. **Setup:** Set `STALE_MINUTES=5` in environment

2. **Action:** Insert row with `updated_at = now() - interval '10 minutes'` and start service

3. **Expected Results:**
   - Row is reset (10 min > 5 min threshold)
   - Log shows recovered row

**Pass Criteria:**
- ✅ Custom threshold is applied
- ✅ Rows older than threshold are reset
- ✅ Rows newer than threshold are unchanged

---

### 7. Lifespan — Pool Cleanup on Startup Failure

**Objective:** Verify database pool is closed if startup fails.

**Test Steps:**

1. **Setup:** Configure invalid migration or break sweep function

2. **Action:** Start service (will fail during lifespan startup)

3. **Expected Results:**
   - Service fails to start
   - Database pool is closed (no connection leak)
   - Error logged with details

**Pass Criteria:**
- ✅ Pool cleanup happens even on startup failure
- ✅ No connection leaks (verify with `SELECT count(*) FROM pg_stat_activity`)
- ✅ Error is logged clearly

---

### 8. Lifespan — Pool Cleanup on Shutdown

**Objective:** Verify database pool is closed on normal shutdown.

**Test Steps:**

1. **Action:** Start service, then send SIGTERM or Ctrl+C

2. **Expected Results:**
   - Service shuts down gracefully
   - Database pool is closed
   - No connection leaks

**Pass Criteria:**
- ✅ Pool cleanup happens on shutdown
- ✅ No connection leaks
- ✅ Shutdown completes within 5 seconds

---

## Integration Test Scenarios

### 9. End-to-End: Crash Recovery

**Objective:** Verify the complete crash recovery flow.

**Test Steps:**

1. **Setup:** Start service and begin indexing a document
   ```bash
   curl -X POST http://localhost:8000/v1/files \
     -H "Content-Type: application/json" \
     -d '{"tos_key": "products/manual.pdf"}'
   ```

2. **Action:** Kill service mid-indexing (SIGKILL)
   ```bash
   kill -9 <pid>
   ```

3. **Verify:** Check database — row should be stuck in `indexing` state
   ```sql
   SELECT status FROM ark_files WHERE tos_key = 'products/manual.pdf';
   -- Expected: 'indexing'
   ```

4. **Action:** Restart service

5. **Expected Results:**
   - Service starts successfully
   - Sweep resets the stuck row to `stored`
   - Log shows: `"Startup sweep recovered 1 stale indexing + 0 stale deindexing rows"`
   - Row can be re-indexed

**Pass Criteria:**
- ✅ Stuck row is automatically recovered
- ✅ No manual intervention needed
- ✅ Row can be re-indexed successfully

---

### 10. End-to-End: VikingDB Retry Under Load

**Objective:** Verify retry logic works under concurrent load.

**Test Steps:**

1. **Setup:** Configure VikingDB with rate limiting (e.g., 10 req/sec)

2. **Action:** Send 50 concurrent indexing requests
   ```bash
   for i in {1..50}; do
     curl -X POST http://localhost:8000/v1/files \
       -H "Content-Type: application/json" \
       -d "{\"tos_key\": \"test-$i.pdf\"}" &
   done
   wait
   ```

3. **Expected Results:**
   - Some requests hit rate limit and retry
   - All 50 requests eventually succeed
   - Retry logs show exponential backoff
   - No requests fail permanently

**Pass Criteria:**
- ✅ All requests succeed (may take longer due to retries)
- ✅ Rate limit errors are retried automatically
- ✅ No permanent failures
- ✅ Logs show retry attempts with delays

---

## Performance Test Scenarios

### 11. Startup Time Impact

**Objective:** Measure sweep overhead on startup time.

**Test Steps:**

1. **Setup:** Database with varying numbers of stale rows (0, 10, 100, 1000)

2. **Action:** Measure startup time for each scenario
   ```bash
   time python -m uvicorn main:app --host 0.0.0.0 --port 8000
   ```

3. **Expected Results:**
   - 0 stale rows: baseline startup time
   - 10 stale rows: +50ms
   - 100 stale rows: +200ms
   - 1000 stale rows: +1s

**Pass Criteria:**
- ✅ Startup time scales linearly with stale row count
- ✅ Overhead is acceptable (<2s for 1000 rows)
- ✅ No timeout or crash with large datasets

---

### 12. Retry Overhead

**Objective:** Measure retry overhead on successful operations.

**Test Steps:**

1. **Setup:** VikingDB with no failures (100% success rate)

2. **Action:** Measure latency for 100 `add_doc` calls

3. **Expected Results:**
   - No retry overhead (first attempt succeeds)
   - Latency matches baseline (no retry logic)

**Pass Criteria:**
- ✅ No measurable overhead when retries aren't needed
- ✅ Latency p50/p95/p99 unchanged

---

## Edge Cases

### 13. Malformed Database Response

**Objective:** Verify sweep handles unexpected database responses.

**Test Steps:**

1. **Setup:** Mock `conn.execute()` to return non-standard format (e.g., `"UPDATED"` instead of `"UPDATE 3"`)

2. **Action:** Start service

3. **Expected Results:**
   - Sweep logs error: `"Failed to parse sweep result 'UPDATED': ..."`
   - Sweep returns 0
   - Service continues startup

**Pass Criteria:**
- ✅ Service doesn't crash on malformed response
- ✅ Error is logged with details
- ✅ Sweep returns 0 (safe default)

---

### 14. Zero STALE_MINUTES

**Objective:** Verify validation of `STALE_MINUTES` setting.

**Test Steps:**

1. **Setup:** Set `STALE_MINUTES=0` in environment

2. **Action:** Start service

3. **Expected Results:**
   - Service fails to start (validation error)
   - Error message: `"STALE_MINUTES must be >= 1"`

**Pass Criteria:**
- ✅ Invalid config is rejected
- ✅ Clear error message
- ✅ Service doesn't start with invalid config

---

### 15. Concurrent Sweep and Indexing

**Objective:** Verify sweep doesn't interfere with active indexing.

**Test Steps:**

1. **Setup:** Start indexing operation that takes 30 seconds

2. **Action:** Restart service while indexing is in progress (row is in `indexing` state but <15 min old)

3. **Expected Results:**
   - Sweep doesn't reset the fresh row (updated_at is recent)
   - Indexing operation can resume or be retried

**Pass Criteria:**
- ✅ Fresh rows are not reset by sweep
- ✅ No race condition between sweep and active operations
- ✅ Indexing can complete successfully

---

## Regression Test Scenarios

### 16. Existing Functionality — No Impact

**Objective:** Verify new features don't break existing functionality.

**Test Steps:**

1. **Action:** Run existing ark-middleware test suite

2. **Expected Results:**
   - All existing tests pass
   - No new failures introduced

**Pass Criteria:**
- ✅ 100% existing test pass rate
- ✅ No performance degradation
- ✅ No API contract changes

---

## Manual Testing Checklist

- [ ] Startup sweep resets stale `indexing` rows
- [ ] Startup sweep resets stale `deindexing` rows
- [ ] Startup sweep logs warning when rows are recovered
- [ ] Startup sweep handles database errors gracefully
- [ ] VikingDB retry recovers from transient failures
- [ ] VikingDB retry exhausts after 3 attempts
- [ ] VikingDB retry uses exponential backoff (1s, 2s, 4s)
- [ ] Custom `STALE_MINUTES` setting is respected
- [ ] Database pool is closed on startup failure
- [ ] Database pool is closed on normal shutdown
- [ ] Service starts successfully with no stale rows
- [ ] Crash recovery flow works end-to-end
- [ ] Retry logic works under concurrent load
- [ ] Startup time overhead is acceptable
- [ ] Existing functionality is not impacted

---

## Test Environment Requirements

### Minimum Requirements

- Python 3.13+
- PostgreSQL 12+ with `ark_files` table
- VikingDB collection (or mock)
- TOS bucket (or mock)

### Recommended Setup

- Docker Compose with PostgreSQL + ark service
- Network proxy for failure injection (e.g., Toxiproxy)
- Load testing tool (e.g., Apache Bench, wrk)
- Monitoring (logs, metrics, traces)

---

## Known Limitations

1. **Broad exception catch:** Retry logic catches all `Exception` types. Should be narrowed to SDK-specific transient errors when available.

2. **No metrics:** Sweep counts and retry attempts are logged but not emitted as metrics. Consider adding Prometheus metrics for observability.

3. **No integration tests:** All tests use mocks. Real PostgreSQL + VikingDB integration tests would increase confidence.

4. **No load tests:** Performance under high concurrency is untested. Recommend load testing before production deployment.

---

## Sign-Off

**Implementation:** ✅ Complete  
**Unit Tests:** ✅ 19/19 passing  
**Code Review:** ✅ Approved  
**Documentation:** ✅ Complete  

**Ready for:** Manual testing, integration testing, staging deployment

**Next Steps:**
1. Execute manual test scenarios (sections 1-15)
2. Run integration tests in staging environment
3. Monitor startup sweep logs for 1 week
4. Collect retry metrics and tune thresholds if needed
5. Deploy to production with gradual rollout
