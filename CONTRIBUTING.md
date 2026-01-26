# Contributing to AfterRun

Thanks for your interest in contributing.

## Project Philosophy

AfterRun is intentionally minimal.

The goal is to detect **silent job failures** reliably and predictably.
Features that add complexity, hidden behavior, or magic are avoided.

Before proposing changes, please keep in mind:

- Explicit rules are preferred over heuristics
- Database writes must be reliable and idempotent
- Alerting must be DB-first and best-effort
- Fail-open is preferred to fail-closed
- Simplicity beats cleverness

## What Contributions Are Welcome

- Bug fixes
- Reliability improvements
- Documentation improvements
- Tests
- Small, well-scoped features that align with the core goal

## What Is Out of Scope

- Workflow orchestration
- Dashboards or UI-heavy changes
- AI / anomaly detection
- Replacing cron or schedulers
- Large refactors without discussion

## How to Contribute

1. Open an issue describing the problem or idea
2. Discuss the approach before coding
3. Keep changes small and focused
4. Prefer clarity over abstraction

## Development Setup

See `README.md` for local setup instructions.

---

This project is early-stage and opinionated.
If something feels missing, open an issue and start the conversation.

