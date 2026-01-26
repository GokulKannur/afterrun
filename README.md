# AfterRun

AfterRun monitors cron jobs and background tasks and alerts when they
**run successfully but produce incorrect results**.

Most monitoring tools only tell you whether a job executed (exit code `0`).
AfterRun tells you whether the job actually did useful work.

This project exists because “it ran” is not the same as “it worked”.

---

## The problem

Cron jobs fail silently.

A job can:
- start on time
- exit cleanly
- send a success ping

…and still fail in ways that matter.

Common failure modes:

- **Zero throughput**  
  The script ran, but processed `0` rows.

- **Empty artifacts**  
  A backup job succeeded but uploaded an empty or truncated file.

- **Logic errors**  
  The script completed too quickly, skipped a loop, or exited early.

- **Stale jobs**  
  The job stopped running days ago, but no explicit error was emitted.

If you only monitor exit codes or heartbeats, you usually find out about
these failures when:
- a restore fails
- reports are wrong
- downstream systems break
- a customer complains

AfterRun is designed to catch these failures early.

---

## What AfterRun does

AfterRun is a small monitoring service for cron jobs and background tasks
that focuses on **job correctness**, not just execution.

It lets you:

- Push custom metrics with each run (rows processed, file size, duration)
- Define simple, explicit threshold rules
- Detect:
  - jobs that ran but produced bad output
  - jobs that stopped running entirely
  - duplicate or retry pings
- Receive alerts via email and Slack (optional)

Alerts are:
- written to the database first (no lost events)
- delivered asynchronously
- deduplicated to avoid noise

---

## How it works

AfterRun uses a push-based model.

Your job reports its result to AfterRun at the end of execution.
AfterRun evaluates the payload against rules you define.

### 1. The job runs

Your script executes and sends a JSON payload containing:

- status (`ok` or `fail`)
- duration
- metrics relevant to the job’s domain

Example metrics:
- `rows_processed`
- `file_size_bytes`
- `records_exported`
- `items_synced`

### 2. The check

AfterRun evaluates the payload using explicit rules such as:

- `rows_processed > 0`
- `file_size_bytes > 1024`
- `duration_ms < 600000`

There is no anomaly detection or “learning” in the core engine.
Rules are deterministic and transparent.

### 3. The alert

An alert is triggered when:
- a rule is violated
- a job explicitly reports failure
- a job stops reporting within its expected window

Alerts are persisted before delivery.

---

## Quick start

You do not need an SDK.
A simple `curl` call is sufficient.

### Bash example

```bash
#!/bin/bash

# Run the actual job
./backup.sh

BACKUP_SIZE=$(wc -c < /tmp/backup.tar.gz)
ROWS=$(cat /tmp/rows_processed.txt)

# Report the result
curl -X POST https://api.afterrun.example/ping/{job_id} \
  -H "Content-Type: application/json" \
  -d "{
    \"status\": \"ok\",
    \"duration_ms\": $SECONDS,
    \"metrics\": {
      \"rows_processed\": $ROWS,
      \"file_size_bytes\": $BACKUP_SIZE
    }
  }"
```

If `rows_processed` is 0, or the backup file is smaller than your
threshold, AfterRun marks the run as failed and sends an alert.

---

## Alert behavior

### Alerts are DB-first
The alert is written to the database **before** email or Slack delivery.

### Alerts are deduplicated
You receive one alert per failure condition, not one per retry.

### Delivery is best-effort
Email or Slack failures do not affect the core system.

### Missed run detection
AfterRun also detects jobs that stop reporting entirely.

If a job does not send a ping within its expected window:
1. a “missed run” alert is generated
2. duplicate alerts are suppressed until the job runs again

This catches:
- broken cron schedules
- disabled servers
- scripts that never start

---

## Design philosophy

### Correctness > execution
A successful exit code is not enough.

### Explicit over implicit
Simple threshold rules are easier to reason about than black-box models.

### Fail safely
Alerts are persisted before delivery. Side effects are isolated.

### Minimal surface area
No dashboards, agents, or invasive integrations required.

### Boring by design
This is infrastructure plumbing. It should be predictable and quiet.

---

## What this is not

AfterRun is **not**:
- a workflow orchestrator
- a metrics or APM platform
- a replacement for schedulers like cron, Airflow, or Prefect

It is a **correctness monitor** for jobs you already run.

---

## Current status

- [x] Core engine implemented
- [x] Threshold-based rule evaluation
- [x] Email alerts
- [x] Optional Slack alerts
- [x] Missed run detection
- [x] Duplicate ping protection
