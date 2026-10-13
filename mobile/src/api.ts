import { Platform } from "react-native";
import Constants from "expo-constants";

const ACCOUNTS = {
  rider: { phone: "+244923000001", password: "DemoPass123!" },
  driver: { phone: "+244923000002", password: "DemoPass123!" },
} as const;

export type Role = keyof typeof ACCOUNTS;

export const FROM = { lat: -9.455, lng: 13.215 };
export const TO = { lat: -9.45, lng: 13.265 };

export function apiBase(): string {
  const override = process.env.EXPO_PUBLIC_API_URL?.replace(/\/$/, "");
  if (override) return override;
  const host = Constants.expoConfig?.hostUri?.split(":")[0];
  if (host && Platform.OS !== "web") {
    return `http://${host}:8080`;
  }
  return "http://127.0.0.1:8080";
}

function cents(v: unknown): number {
  if (v == null) return 0;
  if (typeof v === "number") return v;
  if (typeof v === "string") return Number(v);
  if (typeof v === "object" && v !== null && "Int" in v) {
    const n = v as { Int?: number; Exp?: number };
    return Number(n.Int) * Math.pow(10, Number(n.Exp || 0));
  }
  return Number(v) || 0;
}

export function kz(v: unknown): string {
  return "Kz " + Math.round(cents(v)).toLocaleString("pt-PT");
}

export async function req(
  path: string,
  opts: { method?: string; token?: string; body?: unknown; lang?: "pt" | "en" } = {},
): Promise<any> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "Accept-Language": opts.lang === "en" ? "en" : "pt",
  };
  if (opts.token) headers.Authorization = "Bearer " + opts.token;
  const res = await fetch(apiBase() + "/api/v1" + path, {
    method: opts.method ?? "GET",
    headers,
    body: opts.body ? JSON.stringify(opts.body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.status + " " + path);
  return data;
}

export function pick(obj: any, ...keys: string[]): any {
  if (obj == null) return undefined;
  for (const key of keys) {
    if (obj[key] != null) return obj[key];
    const pascal = key.charAt(0).toUpperCase() + key.slice(1);
    if (obj[pascal] != null) return obj[pascal];
  }
  return undefined;
}

export function addressOf(value: any): string {
  if (!value) return "";
  if (typeof value === "string") return value;
  return String(pick(value, "address") || "");
}

export type PayMethod = "cash" | "multicaixa" | "card";
export type ScreenId =
  | "trips"
  | "payment"
  | "invoices"
  | "notifications"
  | "savedPlaces"
  | "airport"
  | "business"
  | "promo"
  | "safety"
  | "support"
  | "settings"
  | "earnings"
  | "wallet"
  | "kyc"
  | "vehicle";

export async function login(role: Role): Promise<string> {
  const data = await req("/auth/login", { method: "POST", body: ACCOUNTS[role] });
  return data.accessToken as string;
}

export async function quoteFare(): Promise<number> {
  const q = await req(
    `/rides/suggested-fare?fromLat=${FROM.lat}&fromLng=${FROM.lng}&toLat=${TO.lat}&toLng=${TO.lng}`,
  );
  return Math.max(cents(q.totalCents) || 1500, 100);
}
