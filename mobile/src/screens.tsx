import { useEffect, useState } from "react";
import {
  ActivityIndicator,
  Pressable,
  SafeAreaView,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import {
  addressOf,
  kz,
  pick,
  req,
  type PayMethod,
  type Role,
  type ScreenId,
} from "./api";
import type { Copy, Lang } from "./i18n";

type Props = {
  screen: ScreenId;
  token: string;
  role: Role;
  lang: Lang;
  t: Copy;
  payment: PayMethod;
  promoCode: string;
  businessId: string;
  lastRideId: string;
  onBack: () => void;
  onPayment: (method: PayMethod) => void;
  onPromo: (code: string) => void;
  onBusiness: (id: string) => void;
  onPlace: (address: string, asPickup: boolean) => void;
};

export function AccountScreen(props: Props) {
  const titles: Record<ScreenId, string> = {
    trips: props.t.trips,
    payment: props.t.payment,
    invoices: props.t.invoices,
    notifications: props.t.notifications,
    savedPlaces: props.t.savedPlaces,
    airport: props.t.airport,
    business: props.t.business,
    promo: props.t.promo,
    safety: props.t.safety,
    support: props.t.support,
    settings: props.t.settings,
    earnings: props.t.earnings,
    wallet: props.t.wallet,
    kyc: props.t.kyc,
    vehicle: props.t.vehicle,
  };
  return (
    <SafeAreaView style={styles.page}>
      <View style={styles.top}>
        <Pressable onPress={props.onBack} style={styles.backBtn}>
          <Text style={styles.backText}>← {props.t.back}</Text>
        </Pressable>
        <Text style={styles.title}>{titles[props.screen]}</Text>
      </View>
      <ScrollView contentContainerStyle={styles.body}>
        <ScreenBody {...props} />
      </ScrollView>
    </SafeAreaView>
  );
}

function ScreenBody(props: Props) {
  switch (props.screen) {
    case "trips":
      return <TripsScreen {...props} />;
    case "payment":
      return <PaymentScreen {...props} />;
    case "invoices":
      return <InvoicesScreen {...props} />;
    case "notifications":
      return <NotesScreen {...props} />;
    case "savedPlaces":
      return <PlacesScreen {...props} />;
    case "airport":
      return <AirportScreen {...props} />;
    case "business":
      return <BusinessScreen {...props} />;
    case "promo":
      return <PromoScreen {...props} />;
    case "safety":
      return <SafetyScreen {...props} />;
    case "support":
      return <SupportScreen {...props} />;
    case "settings":
      return <SettingsScreen {...props} />;
    case "earnings":
      return <EarningsScreen {...props} />;
    case "wallet":
      return <WalletScreen {...props} />;
    case "kyc":
      return <KycScreen {...props} />;
    case "vehicle":
      return <VehicleScreen {...props} />;
  }
}

function useLoad<T>(token: string, fn: () => Promise<T>) {
  const [data, setData] = useState<T | null>(null);
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(true);
  async function reload() {
    try {
      setErr("");
      setLoading(true);
      setData(await fn());
    } catch (e: any) {
      setErr(String(e.message || e));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    reload();
  }, [token]);
  return { data, err, loading, reload, setData };
}

function Status({ t, loading, err, onRetry, empty }: { t: Copy; loading: boolean; err: string; onRetry: () => void; empty?: boolean }) {
  if (loading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator />
        <Text style={styles.meta}>{t.loading}</Text>
      </View>
    );
  }
  if (err) {
    return (
      <View>
        <Text style={styles.err}>{err}</Text>
        <Pressable style={styles.cta} onPress={onRetry}>
          <Text style={styles.ctaText}>{t.retry}</Text>
        </Pressable>
      </View>
    );
  }
  if (empty) return <Text style={styles.meta}>{t.empty}</Text>;
  return null;
}

function Card({ title, meta, onPress, active }: { title: string; meta?: string; onPress?: () => void; active?: boolean }) {
  return (
    <Pressable style={[styles.card, active && styles.cardOn]} onPress={onPress} disabled={!onPress}>
      <Text style={styles.cardTitle}>{title}</Text>
      {meta ? <Text style={styles.meta}>{meta}</Text> : null}
    </Pressable>
  );
}

function TripsScreen({ token, t, lastRideId, onBack }: Props) {
  const load = useLoad(token, async () => (await req("/rides", { token })).rides || []);
  const rides = load.data || [];
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} empty={!rides.length} />
      {rides.map((ride: any) => (
        <Card
          key={pick(ride, "id")}
          title={`${pick(ride, "status")} · ${kz(pick(ride, "acceptedFareCents") || pick(ride, "suggestedFareCents"))}`}
          meta={`${addressOf(pick(ride, "pickup"))} → ${addressOf(pick(ride, "destination"))}`}
        />
      ))}
      {lastRideId ? <Text style={styles.meta}>ID {lastRideId}</Text> : null}
      <Pressable style={styles.ghost} onPress={onBack}>
        <Text style={styles.ghostText}>{t.newTrip}</Text>
      </Pressable>
    </>
  );
}

function PaymentScreen({ t, payment, onPayment }: Props) {
  const methods: PayMethod[] = ["cash", "multicaixa", "card"];
  const label: Record<PayMethod, string> = { cash: t.cash, multicaixa: t.multicaixa, card: t.card };
  return (
    <>
      <Text style={styles.meta}>{t.defaultPay}</Text>
      {methods.map((method) => (
        <Card
          key={method}
          title={label[method]}
          meta={payment === method ? t.selected : undefined}
          active={payment === method}
          onPress={() => onPayment(method)}
        />
      ))}
    </>
  );
}

function InvoicesScreen({ token, t }: Props) {
  const load = useLoad(token, async () => {
    const dash = await req("/riders/dashboard", { token });
    return { invoices: dash.invoices || [], rides: dash.recentRides || [] };
  });
  const invoices = load.data?.invoices || [];
  const rides = (load.data?.rides || []).filter((r: any) => pick(r, "status") === "completed");
  async function issue(rideId: string) {
    await req("/rides/" + rideId + "/invoice", { method: "POST", token });
    load.reload();
  }
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} empty={!invoices.length && !rides.length} />
      {invoices.map((inv: any) => (
        <Card
          key={pick(inv, "id") || pick(inv, "ID")}
          title={String(pick(inv, "invoiceNumber") || pick(inv, "InvoiceNumber") || t.invoices)}
          meta={String(pick(inv, "status") || pick(inv, "Status") || "")}
        />
      ))}
      {rides.map((ride: any) => (
        <Pressable key={pick(ride, "id")} style={styles.cta} onPress={() => issue(pick(ride, "id"))}>
          <Text style={styles.ctaText}>
            {t.invoiceRide} · {addressOf(pick(ride, "destination") || pick(ride, "DestinationAddress"))}
          </Text>
        </Pressable>
      ))}
    </>
  );
}

function NotesScreen({ token, t }: Props) {
  const [tab, setTab] = useState<"unread" | "read">("unread");
  const load = useLoad(token, async () => {
    const unread = await req("/notifications/unread", { token });
    const read = await req("/notifications/read", { token });
    return { unread: unread.notifications || [], read: read.notifications || [] };
  });
  const items = tab === "unread" ? load.data?.unread || [] : load.data?.read || [];
  async function mark(id: string) {
    await req("/notifications/" + id + "/read", { method: "POST", token });
    load.reload();
  }
  return (
    <>
      <View style={styles.row}>
        <Pressable style={[styles.chip, tab === "unread" && styles.chipOn]} onPress={() => setTab("unread")}>
          <Text style={[styles.chipText, tab === "unread" && styles.chipTextOn]}>{t.unread}</Text>
        </Pressable>
        <Pressable style={[styles.chip, tab === "read" && styles.chipOn]} onPress={() => setTab("read")}>
          <Text style={[styles.chipText, tab === "read" && styles.chipTextOn]}>{t.read}</Text>
        </Pressable>
      </View>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} empty={!items.length} />
      {items.map((note: any) => (
        <Card
          key={note.id}
          title={note.title}
          meta={note.body}
          onPress={tab === "unread" ? () => mark(note.id) : undefined}
        />
      ))}
    </>
  );
}

function PlacesScreen({ token, t, onPlace, onBack }: Props) {
  const load = useLoad(token, async () => (await req("/riders/saved-places", { token })).savedPlaces || []);
  const places = load.data || [];
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} empty={!places.length} />
      {places.map((place: any) => {
        const name = String(pick(place, "name") || "");
        const address = String(pick(place, "address") || "");
        const icon = String(pick(place, "icon") || "");
        const asPickup = icon === "home" || icon === "work";
        return (
          <Card
            key={pick(place, "id")}
            title={name}
            meta={address}
            onPress={() => {
              onPlace(address || name, asPickup);
              onBack();
            }}
          />
        );
      })}
    </>
  );
}

function AirportScreen({ token, t, onPlace, onBack }: Props) {
  const load = useLoad(token, async () => {
    const airports = await req("/airport/airports", { token });
    const transfers = await req("/airport/transfers", { token }).catch(() => ({ transfers: [] }));
    return { airports: airports.airports || [], transfers: transfers.transfers || [] };
  });
  const airports = load.data?.airports || [];
  const transfers = load.data?.transfers || [];
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} empty={!airports.length && !transfers.length} />
      <Text style={styles.section}>{t.airports}</Text>
      {airports.map((airport: any) => (
        <Card
          key={airport.id}
          title={`${airport.iata} · ${airport.name}`}
          meta={airport.city}
          onPress={() => {
            onPlace(airport.name, false);
            onBack();
          }}
        />
      ))}
      <Text style={styles.section}>{t.transfers}</Text>
      {transfers.map((item: any) => (
        <Card
          key={item.id}
          title={item.flight_number || item.status}
          meta={`${item.status} · ${item.passenger_name || ""}`}
        />
      ))}
    </>
  );
}

function BusinessScreen({ token, t, businessId, onBusiness }: Props) {
  const [name, setName] = useState("RideX Empresa");
  const load = useLoad(token, async () => (await req("/business/accounts", { token })).accounts || []);
  const accounts = load.data || [];
  async function create() {
    await req("/business/accounts", { method: "POST", token, body: { name, currency: "AOA" } });
    load.reload();
  }
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} />
      {accounts.map((account: any) => {
        const id = String(pick(account, "id") || pick(account, "ID") || "");
        return (
          <Card
            key={id}
            title={String(pick(account, "name") || pick(account, "Name") || t.business)}
            meta={businessId === id ? t.selected : t.useCompany}
            active={businessId === id}
            onPress={() => onBusiness(businessId === id ? "" : id)}
          />
        );
      })}
      <TextInput style={styles.input} value={name} onChangeText={setName} placeholder={t.companyName} />
      <Pressable style={styles.cta} onPress={create}>
        <Text style={styles.ctaText}>{t.createCompany}</Text>
      </Pressable>
    </>
  );
}

function PromoScreen({ token, t, promoCode, onPromo }: Props) {
  const load = useLoad(token, async () => {
    const promos = await req("/riders/promos", { token });
    const loyalty = await req("/riders/loyalty", { token });
    return { promos: promos.promos || [], loyalty: loyalty.loyalty };
  });
  const promos = load.data?.promos || [];
  const loyalty = load.data?.loyalty;
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} empty={!promos.length} />
      {loyalty ? (
        <Card
          title={`${t.loyalty} · ${pick(loyalty, "tier") || "bronze"}`}
          meta={`${pick(loyalty, "balance") || 0} pts`}
        />
      ) : null}
      {promos.map((promo: any) => (
        <Card
          key={promo.code}
          title={promo.code}
          meta={promo.alreadyUsed ? t.done : t.applyPromo}
          active={promoCode === promo.code}
          onPress={() => onPromo(promoCode === promo.code ? "" : promo.code)}
        />
      ))}
    </>
  );
}

function SafetyScreen({ token, t }: Props) {
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("+2449");
  const [relationship, setRelationship] = useState("family");
  const load = useLoad(token, async () => (await req("/emergency-contacts", { token })).contacts || []);
  const contacts = load.data || [];
  async function add() {
    await req("/emergency-contacts", {
      method: "POST",
      token,
      body: { name, phone, relationship },
    });
    setName("");
    load.reload();
  }
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} empty={!contacts.length} />
      {contacts.map((contact: any) => (
        <Card key={contact.id} title={contact.name} meta={`${contact.relationship} · ${contact.phone}`} />
      ))}
      <TextInput style={styles.input} value={name} onChangeText={setName} placeholder={t.contactName} />
      <TextInput style={styles.input} value={phone} onChangeText={setPhone} placeholder={t.contactPhone} />
      <TextInput style={styles.input} value={relationship} onChangeText={setRelationship} placeholder={t.relationship} />
      <Pressable style={styles.cta} onPress={add}>
        <Text style={styles.ctaText}>{t.addContact}</Text>
      </Pressable>
    </>
  );
}

function SupportScreen({ token, t, lastRideId }: Props) {
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");
  const [msg, setMsg] = useState("");
  const load = useLoad(token, async () => (await req("/support-tickets", { token })).tickets || []);
  const tickets = load.data || [];
  async function open() {
    const rideId = lastRideId || pick((await req("/rides", { token })).rides?.[0], "id");
    if (!rideId) throw new Error(t.empty);
    await req("/rides/" + rideId + "/support-tickets", {
      method: "POST",
      token,
      body: { category: "general", subject, description },
    });
    setSubject("");
    setDescription("");
    setMsg(t.save);
    load.reload();
  }
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} empty={!tickets.length} />
      {tickets.map((ticket: any) => (
        <Card key={ticket.id} title={ticket.subject} meta={`${ticket.status} · ${ticket.category}`} />
      ))}
      <TextInput style={styles.input} value={subject} onChangeText={setSubject} placeholder={t.ticketSubject} />
      <TextInput style={styles.input} value={description} onChangeText={setDescription} placeholder={t.ticketBody} />
      <Pressable style={styles.cta} onPress={() => open().catch((e) => setMsg(String(e.message || e)))}>
        <Text style={styles.ctaText}>{t.openTicket}</Text>
      </Pressable>
      {msg ? <Text style={styles.meta}>{msg}</Text> : null}
    </>
  );
}

function SettingsScreen({ token, t, lang }: Props) {
  const load = useLoad(token, async () => {
    const me = await req("/auth/me", { token });
    const identity = await req("/identity/verification", { token }).catch(() => null);
    return { me, identity };
  });
  const me = load.data?.me;
  const identity = load.data?.identity?.verification || load.data?.identity;
  async function logout() {
    await req("/auth/logout", { method: "POST", token }).catch(() => {});
  }
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} />
      {me ? <Card title={me.name} meta={`${me.phone} · ${me.role} · ${me.rating}`} /> : null}
      <Card title={t.language} meta={lang === "pt" ? t.portuguese : t.english} />
      <Card title={t.identity} meta={pick(identity, "status") || t.empty} />
      <Text style={styles.meta}>{t.demoSession}</Text>
      <Pressable style={styles.ghost} onPress={logout}>
        <Text style={styles.ghostText}>{t.logout}</Text>
      </Pressable>
    </>
  );
}

function EarningsScreen({ token, t }: Props) {
  const load = useLoad(token, async () => (await req("/drivers/earnings", { token })).earnings);
  const e = load.data;
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} empty={!e} />
      {e ? (
        <>
          <Card title={t.completedRides} meta={String(e.completedRides ?? 0)} />
          <Card title={t.gross} meta={kz(e.grossFares)} />
          <Card title={t.cash} meta={kz(e.cashEarnings)} />
          <Card title={t.digital} meta={kz(e.digitalEarnings)} />
          <Card title={t.unpaid} meta={kz(e.unpaidFares)} />
        </>
      ) : null}
    </>
  );
}

function WalletScreen({ token, t }: Props) {
  const load = useLoad(token, async () => {
    const wallet = await req("/drivers/wallet", { token }).catch(() => ({ wallet: null }));
    const tx = await req("/drivers/wallet/transactions", { token }).catch(() => ({ transactions: [] }));
    const payouts = await req("/drivers/payouts", { token }).catch(() => ({ payouts: [] }));
    return { wallet: wallet.wallet, tx: tx.transactions || [], payouts: payouts.payouts || [] };
  });
  const [msg, setMsg] = useState("");
  const wallet = load.data?.wallet;
  async function withdraw() {
    try {
      setMsg("");
      await req("/drivers/payouts", {
        method: "POST",
        token,
        body: { amountCents: 5000, method: "multicaixa" },
      });
      load.reload();
    } catch (e: any) {
      setMsg(String(e.message || e));
    }
  }
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} />
      {wallet ? (
        <>
          <Card title={t.balance} meta={kz(wallet.balanceCents)} />
          <Card title={t.pending} meta={kz(wallet.pendingCents)} />
        </>
      ) : null}
      <Text style={styles.meta}>{t.withdrawHint}</Text>
      <Pressable style={styles.cta} onPress={withdraw}>
        <Text style={styles.ctaText}>{t.withdraw}</Text>
      </Pressable>
      {msg ? <Text style={styles.err}>{msg}</Text> : null}
      {(load.data?.tx || []).map((row: any) => (
        <Card key={row.id} title={`${row.type} · ${kz(row.amountCents)}`} meta={row.status} />
      ))}
    </>
  );
}

function KycScreen({ token, t }: Props) {
  const [idDocument, setIdDocument] = useState("Bilhete 000000LA041");
  const [licenseNumber, setLicenseNumber] = useState("AO-DRV-002");
  const [vehiclePlate, setVehiclePlate] = useState("LD-00-02-AA");
  const load = useLoad(token, async () => {
    try {
      return await req("/drivers/kyc", { token });
    } catch {
      return null;
    }
  });
  const kyc = load.data?.kyc || load.data;
  async function submit() {
    await req("/drivers/kyc", {
      method: "POST",
      token,
      body: { idDocument, licenseNumber, vehiclePlate },
    });
    load.reload();
  }
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} />
      {kyc ? (
        <Card
          title={`${t.kycStatus}: ${pick(kyc, "status") || pick(kyc, "Status") || ""}`}
          meta={`${pick(kyc, "licenseNumber") || pick(kyc, "LicenseNumber") || ""} · ${pick(kyc, "vehiclePlate") || pick(kyc, "VehiclePlate") || ""}`}
        />
      ) : null}
      <TextInput style={styles.input} value={idDocument} onChangeText={setIdDocument} placeholder={t.idDocument} />
      <TextInput style={styles.input} value={licenseNumber} onChangeText={setLicenseNumber} placeholder={t.license} />
      <TextInput style={styles.input} value={vehiclePlate} onChangeText={setVehiclePlate} placeholder={t.plate} />
      <Pressable style={styles.cta} onPress={submit}>
        <Text style={styles.ctaText}>{t.submitKyc}</Text>
      </Pressable>
    </>
  );
}

function VehicleScreen({ token, t }: Props) {
  const [make, setMake] = useState("Toyota");
  const [model, setModel] = useState("Corolla");
  const [year, setYear] = useState("2019");
  const [color, setColor] = useState("Branco");
  const [plate, setPlate] = useState("LD-00-02-AA");
  const load = useLoad(token, async () => {
    try {
      return (await req("/drivers/vehicle-profile", { token })).vehicleProfile;
    } catch {
      return null;
    }
  });
  const profile = load.data;
  async function save() {
    const body = { make, model, year: Number(year), color, licensePlate: plate };
    try {
      await req("/drivers/vehicle-profile", { method: "PUT", token, body });
    } catch {
      await req("/drivers/vehicle-profile", { method: "POST", token, body });
    }
    load.reload();
  }
  return (
    <>
      <Status t={t} loading={load.loading} err={load.err} onRetry={load.reload} />
      {profile ? (
        <Card
          title={`${profile.make} ${profile.model}`}
          meta={`${profile.year} · ${profile.color} · ${profile.licensePlate}`}
        />
      ) : null}
      <TextInput style={styles.input} value={make} onChangeText={setMake} placeholder={t.make} />
      <TextInput style={styles.input} value={model} onChangeText={setModel} placeholder={t.model} />
      <TextInput style={styles.input} value={year} onChangeText={setYear} placeholder={t.year} />
      <TextInput style={styles.input} value={color} onChangeText={setColor} placeholder={t.color} />
      <TextInput style={styles.input} value={plate} onChangeText={setPlate} placeholder={t.plate} />
      <Pressable style={styles.cta} onPress={save}>
        <Text style={styles.ctaText}>{t.saveVehicle}</Text>
      </Pressable>
    </>
  );
}

const styles = StyleSheet.create({
  page: { flex: 1, backgroundColor: "#fff" },
  top: { paddingHorizontal: 18, paddingTop: 8, paddingBottom: 12, borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: "#ececec" },
  backBtn: { marginBottom: 8 },
  backText: { fontWeight: "700", color: "#111" },
  title: { fontSize: 28, fontWeight: "800" },
  body: { padding: 18, paddingBottom: 40, gap: 10 },
  center: { alignItems: "center", paddingVertical: 24, gap: 8 },
  meta: { color: "#6b7280", marginTop: 4 },
  err: { color: "#b91c1c", marginTop: 8 },
  section: { fontSize: 12, fontWeight: "700", color: "#6b7280", letterSpacing: 0.6, marginTop: 8 },
  card: { backgroundColor: "#f7f7f8", borderRadius: 16, padding: 14, borderWidth: 2, borderColor: "transparent" },
  cardOn: { borderColor: "#111", backgroundColor: "#fff" },
  cardTitle: { fontWeight: "800", fontSize: 16 },
  input: { backgroundColor: "#f4f5f7", borderRadius: 14, padding: 14, fontWeight: "600", fontSize: 16 },
  cta: { backgroundColor: "#111", borderRadius: 16, padding: 16, alignItems: "center", marginTop: 4 },
  ctaText: { color: "#fff", fontWeight: "800" },
  ghost: { backgroundColor: "#f3f4f6", borderRadius: 16, padding: 16, alignItems: "center", marginTop: 4 },
  ghostText: { fontWeight: "800" },
  row: { flexDirection: "row", gap: 8, marginBottom: 8 },
  chip: { paddingHorizontal: 14, paddingVertical: 8, borderRadius: 999, backgroundColor: "#f3f4f6" },
  chipOn: { backgroundColor: "#111" },
  chipText: { fontWeight: "700", color: "#6b7280" },
  chipTextOn: { color: "#fff" },
});
