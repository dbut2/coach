# Database Schema

The canonical table reference is [`db/schema.sql`](../db/schema.sql), regenerated from the goose migrations by `tools/schemadump` (it replays `db/migrations` into a throwaway Postgres and dumps the result).
sqlc (`db/sqlc.yaml`) generates the Go layer in `go/database` from the migrations.

## Overview

The relationships below are the foreign keys declared in the migration; the
tables-only `schema.sql` does not carry them.

```mermaid
erDiagram
    users ||--o{ activities : has
    users ||--o{ messages : has
    users ||--o{ runner_facts : has
    users ||--o{ plans : has
    users ||--o{ planned_workouts : has
    users ||--o{ plan_change_proposals : has
    users ||--o{ wellness_metrics : has
    users ||--o| garmin_connections : has
    users ||--o| conversation_summaries : has

    activities ||--o| activity_streams : "streams (cascade)"
    activities |o--o{ planned_workouts : "completed by"

    messages ||--o| conversation_summaries : watermark
    messages ||--o{ runner_facts : "sourced from"
    messages |o--o{ plan_change_proposals : "triggered by"

    runner_facts |o--o{ plans : goal

    plans ||--o{ planned_workouts : "contains (cascade)"
    plans ||--o{ plan_change_proposals : "proposed for"
```
