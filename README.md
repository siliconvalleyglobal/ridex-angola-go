# RideX Angola (Go)

HTTP API, matching loop, and background worker for RideX Angola.

## Local studio (web + mobile)

This repository is the backend. A local studio UI is served by the API so you
can watch the same live data on a web ops console and on rider/driver phone
shells:

```bash
./scripts/dev-local.sh
```

Then open:

- Web + phones: http://127.0.0.1:8080/app
- Mobile alias: http://127.0.0.1:8080/mobile

Demo accounts (password `DemoPass123!`):

| Role | Phone |
| --- | --- |
| Admin | `+244923000000` |
| Rider | `+244923000001` |
| Driver | `+244923000002` |

The rider and driver columns are phone-sized. Use them as the mobile apps.
There is no separate Expo/Play client in this repo.

## Processes

`dev-local.sh` starts PostgreSQL, Redis (Valkey-compatible), applies
migrations, seeds the demo accounts, then runs `cmd/api`, `cmd/matching`, and
`cmd/worker`.

## Validation

```bash
sqlc generate
gofmt -w .
go test ./...
go vet ./...
```
