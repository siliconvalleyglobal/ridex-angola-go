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

## Testing on iPhone

This repository is Go only. There is no `.xcodeproj`, Expo app, or App Store
binary here, so Xcode Simulator cannot run a native RideX iOS client from
this tree.

To test the current rider/driver UI on a real iPhone:

1. On your Mac, start the backend: `./scripts/dev-local.sh`
2. Find the Mac LAN address (`ipconfig getifaddr en0`, e.g. `192.168.1.20`)
3. Confirm the API is reachable: `http://<mac-ip>:8080/health`
4. On the iPhone (same Wi-Fi), open Safari to `http://<mac-ip>:8080/app`
5. Optional: Share → **Add to Home Screen** for a full-screen RideX icon

Safari talks to `/app` on the same origin, so CORS does not apply. If the
phone cannot load the page, allow incoming connections on port 8080 in
macOS Firewall.

A native iOS app (Swift or Expo) would be a **separate client** that calls
this API. Point it at `http://<mac-ip>:8080/api/v1`, set
`CORS_ALLOWED_ORIGINS` to that app's origin if it is a WebView, and run it
from a Mac with Xcode. This Linux environment cannot run the iOS Simulator.

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
