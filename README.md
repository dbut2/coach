# Coach

An AI running coach you text every day. It connects to Strava (with optional
Garmin wellness data), watches your training as it syncs, builds and adapts a
plan around a race goal, and remembers what matters across days — a coach that
happens to see all your data, not a dashboard.

<p align="center">
  <img src="/docs/screenshots/login.png" width="250" />
  <img src="/docs/screenshots/conversation.png" width="250" />
  <img src="/docs/screenshots/settings.png" width="250" />
</p>

## Stack

- **Go 1.26**, [Gin](https://github.com/gin-gonic/gin) HTTP server
- **Anthropic Claude** via [`anthropic-sdk-go`](https://github.com/anthropics/anthropic-sdk-go), agent loop on [Google ADK](https://google.golang.org/adk)
- **PostgreSQL 18**, [sqlc](https://sqlc.dev) queries, [goose](https://github.com/pressly/goose) migrations
- **[templ](https://templ.guide) + [HTMX](https://htmx.org) + SSE**, styled with Tailwind/DaisyUI and [Lucide](https://lucide.dev) icons
- **OpenTelemetry** for tracing and metrics

## Running it

The whole stack runs under Docker Compose (app + Postgres; migrations run on
startup), serving on port `8080`:

```sh
docker compose up --build
```

You'll need a Strava API application and an Anthropic API key, supplied via
environment (Compose reads a local `.env`; never commit secrets). Code generation
(sqlc, templ, the Strava client) is driven by the `Makefile`.

## Documentation

- [`docs/architecture.md`](docs/architecture.md) — how the system is built today
- [`docs/schema.md`](docs/schema.md) — database schema and relationships
- [`docs/brief.md`](docs/brief.md) — original design rationale and decision log
