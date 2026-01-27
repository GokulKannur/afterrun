# Architecture

AfterRun is a push-based cron monitoring system designed for correctness, not just execution.

## Technology Stack

- **Language**: Go 1.22+ (chosen for performance, type safety, and single-binary deployment).
- **Database**: PostgreSQL 15+ (chosen for reliability, ACID transactions, and JSONB support).
- **Web Framework**: Gin (minimal, fast).
- **Frontend**: Go Templates + Static CSS (zero-build step, server-side rendered).
- **Container**: Docker + Docker Compose (standardized dev/prod env).

## Data Flow

1.  **Ping**: Job sends HTTP POST to `/ping/:key`.
2.  **Handler**: `handlers/webhook.go` parses payload.
3.  **DB**: Run is inserted into `job_runs`.
4.  **Rule Engine**: `rules` for the job are fetched and evaluated against the run's `metrics`.
5.  **Alerting**: If a rule fails, an alert is inserted into `alerts`.
6.  **Notification**: (Future) Async worker picks up alerts and sends to Slack/Email.

## Missed Run Detection

- A background goroutine in `main.go`.
- Wakes up every 30 seconds (configurable).
- Queries `jobs` where `last_run + grace_minutes < now`.
- Inserts "Missed Run" alert if detecting a gap.

## License

AGPL-3.0.
