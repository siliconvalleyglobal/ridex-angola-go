# RideX Angola (Go)

HTTP API, matching loop, and background worker for RideX Angola.

## Local studio (web + mobile)

This repository is the backend. The API also serves a map-first RideX studio
at `/app` so you can run the rider and driver flows in a browser the way
Uber, Grab, and inDrive do: full-screen map, bottom sheet, fare bid, and a
driver GO button.

```bash
./scripts/dev-local.sh
```

Then open:

- App: http://127.0.0.1:8080/app
- Mobile alias: http://127.0.0.1:8080/mobile

Use **Passageiro** / **Motorista** to switch roles on the same map. There is
no separate Expo/Play client in this repo.

Demo accounts (password `DemoPass123!`):

| Role | Phone |
| --- | --- |
| Admin | `+244923000000` |
| Rider | `+244923000001` |
| Driver | `+244923000002` |

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
