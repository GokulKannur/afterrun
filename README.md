# AfterRun

**AfterRun catches silent cron failures.**

Most cron monitors tell you *whether a job ran*.  
AfterRun tells you *whether it actually did something useful*.

---

## The Problem

Cron jobs fail quietly all the time:

- Backups run but write **0 bytes**
- ETL jobs run but process **0 rows**
- Scripts succeed but exit early
- Jobs stop running entirely and nobody notices

Traditional tools answer:

> “Did it run?”

That’s the wrong question.

---

## What AfterRun Does

AfterRun monitors **job correctness**, not just execution.

Your jobs send a simple HTTP ping **after they finish**, including:
- Status
- Duration
- Custom metrics (JSON)
- STDERR output (optional)

AfterRun evaluates **explicit rules** and alerts you when something looks wrong.

Examples:
- `rows_processed == 0`
- `file_size_bytes < 1_000_000`
- Job hasn’t reported in 24 hours
- Duplicate retries suppressed safely

---

## How It Works

1. Create a job (with a unique ping key)
2. Your cron job runs
3. It sends a POST request with metrics
4. AfterRun evaluates rules
5. You get alerted **only when it matters**

No agents.  
No SDK required.  
Just HTTP.

---

## Quick Example

### Cron job

```bash
curl -X POST https://your-afterrun-host/ping/abc123 \
  -H "Content-Type: application/json" \
  -d '{
    "status": "ok",
    "duration_ms": 4200,
    "metrics": {
      "rows_processed": 0,
      "file_size_bytes": 2048
    },
    "stderr": "Backup completed but no data found"
  }'
