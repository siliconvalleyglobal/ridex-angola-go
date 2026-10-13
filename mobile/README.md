# RideX Expo app

React Native client for the RideX Angola Go API. Expo compiles it for
**iOS** (via Xcode), **Android**, and web.

## Run on iOS

1. Start the API in another terminal: `./scripts/dev-local.sh`
2. From this folder:

```bash
bun install
bun run ios
```

That opens **Expo Go** or the iOS Simulator. Xcode is required for the
Simulator (`npx expo run:ios` / `bunx expo run:ios` after `xcode-select`).

On a physical iPhone: install **Expo Go**, scan the QR from `bun start`,
same Wi-Fi as the Mac. The app talks to `http://<mac-ip>:8080`.

Override the API:

```bash
EXPO_PUBLIC_API_URL=http://192.168.1.20:8080 bun start
```

## Demo accounts

Password `DemoPass123!`. The app logs in automatically.

| Role | Phone |
| --- | --- |
| Rider | `+244923000001` |
| Driver | `+244923000002` |
