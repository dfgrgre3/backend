# Thanawy Backend

Go backend services, database migrations, generated API contracts, and a small set
of Node-based database utilities.

## Repository layout

| Path | Purpose |
| --- | --- |
| `cmd/` | Deployable and one-off Go entry points. |
| `internal/` | Application code; not importable outside this module. |
| `pkg/` | Reusable, externally importable Go packages. |
| `tools/` | Developer and deployment utilities that are not application entry points. |
| `scripts/` | Operational commands intended to be run directly by developers or operators. |
| `docs/` | API documentation and project documentation. |
| `prisma/`, `supabase/`, `internal/db/migrations/` | Database schemas and migration histories. |
| `proto/`, `src/gen/`, `internal/proto/` | Protocol definitions and generated clients/servers. |

## Common commands

```powershell
go test ./...
go run ./cmd/api
go run ./tools/dev/generate-token
node ./tools/deployment/sync-vercel-env.mjs
```

Keep generated binaries, logs, build caches, and local environment files out of
the repository root. Put new runnable application binaries in `cmd/`, and put
development-only helpers in `tools/`.
