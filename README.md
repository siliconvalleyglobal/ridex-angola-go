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

Use **Passageiro** / **Motorista** to switch roles on the same map.

## Expo iOS / Android app

`mobile/` is an Expo SDK 57 app that calls this API. **Use Expo**, not a
hand-written Xcode project. Xcode is still needed on a Mac to run the
**iOS Simulator**.

```bash
./scripts/dev-local.sh          # API on :8080
cd mobile && bun install && bun run ios
```

- iPhone Simulator: `bun run ios` (Xcode + Simulator installed)
- Physical iPhone: `bun start`, then scan the QR with **Expo Go** (same Wi-Fi)
- Web preview: `bun run web`

On a device, the app uses the Metro host IP and port `8080`. Override with
`EXPO_PUBLIC_API_URL=http://<mac-ip>:8080`.

Demo accounts (password `DemoPass123!`):

| Role | Phone |
| --- | --- |
| Admin | `+244923000000` |
| Rider | `+244923000001` |
| Driver | `+244923000002` |

## Testing on iPhone (Safari fallback)

If you only want the HTML studio, open `http://<mac-ip>:8080/app` in Safari
and optionally Share → **Add to Home Screen**.

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
