import { StatusBar } from "expo-status-bar";
import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Modal,
  Pressable,
  SafeAreaView,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import {
  FROM,
  TO,
  kz,
  login,
  quoteFare,
  req,
  type PayMethod,
  type Role,
  type ScreenId,
} from "./src/api";
import { type Copy, copy, type Lang } from "./src/i18n";
import { AccountScreen } from "./src/screens";

type Product = "go" | "comfort";
type TripStep = "arrive" | "start" | "complete" | "done";

export default function App() {
  const [lang, setLang] = useState<Lang>("pt");
  const [menuOpen, setMenuOpen] = useState(false);
  const [screen, setScreen] = useState<ScreenId | null>(null);
  const [screenToken, setScreenToken] = useState("");
  const t = useMemo(() => copy[lang], [lang]);
  const [role, setRole] = useState<Role>("rider");
  const [tokens, setTokens] = useState({ rider: "", driver: "" });
  const [payment, setPayment] = useState<PayMethod>("cash");
  const [promoCode, setPromoCode] = useState("");
  const [businessId, setBusinessId] = useState("");
  const [unread, setUnread] = useState(0);
  const [quoteCents, setQuoteCents] = useState(1500);
  const [product, setProduct] = useState<Product>("go");
  const [pickup, setPickup] = useState("Marginal de Luanda, Baixa");
  const [dropoff, setDropoff] = useState("Talatona Convention Centre");
  const [rideId, setRideId] = useState("");
  const [offerRideId, setOfferRideId] = useState("");
  const [online, setOnline] = useState(false);
  const [searching, setSearching] = useState(false);
  const [rideStatus, setRideStatus] = useState("");
  const [vehicle, setVehicle] = useState<any>(null);
  const [cancelReason, setCancelReason] = useState("");
  const [tripPin, setTripPin] = useState("");
  const [pinInput, setPinInput] = useState("");
  const [rating, setRating] = useState(5);
  const [rated, setRated] = useState(false);
  const [hasOffer, setHasOffer] = useState(false);
  const [onTrip, setOnTrip] = useState(false);
  const [tripStep, setTripStep] = useState<TripStep>("arrive");
  const [err, setErr] = useState("");

  const fare = product === "comfort" ? Math.round(quoteCents * 1.25) : quoteCents;
  const accountName = role === "driver" ? "Dário Driver" : "Rita Rider";
  const payLabel = payment === "card" ? t.card : payment === "multicaixa" ? t.multicaixa : t.cash;
  const rideActive = ["matched", "driver_arriving", "in_progress"].includes(rideStatus);
  const riderCanCancel = ["requested", "matched", "driver_arriving"].includes(rideStatus);
  const rideFinished = rideStatus === "completed" || rideStatus === "cancelled";
  const riderStatusLabel =
    rideStatus === "matched"
      ? t.driverComing
      : rideStatus === "driver_arriving"
        ? t.yourRideArrived
        : rideStatus === "in_progress"
          ? t.inProgress
          : rideStatus === "completed"
            ? t.done
            : rideStatus === "cancelled"
              ? t.rideCancelled
              : t.finding;
  const vehicleText = vehicle
    ? [`${vehicle.make || ""} ${vehicle.model || ""}`.trim(), vehicle.color, vehicle.licensePlate]
        .filter(Boolean)
        .join(" · ")
    : t.driver;
  const riderItems: { id: ScreenId; title: string; hint?: string }[] = [
    { id: "trips", title: t.trips },
    { id: "payment", title: t.payment, hint: t.paymentHint },
    { id: "invoices", title: t.invoices },
    { id: "notifications", title: unread ? `${t.notifications} · ${unread}` : t.notifications },
    { id: "savedPlaces", title: t.savedPlaces },
    { id: "airport", title: t.airport },
    { id: "business", title: t.business },
    { id: "promo", title: t.promo },
    { id: "safety", title: t.safety, hint: t.safetyHint },
    { id: "support", title: t.support },
    { id: "settings", title: t.settings },
  ];
  const driverItems: { id: ScreenId; title: string; hint?: string }[] = [
    { id: "trips", title: t.trips },
    { id: "earnings", title: t.earnings },
    { id: "wallet", title: t.wallet },
    { id: "kyc", title: t.kyc },
    { id: "vehicle", title: t.vehicle },
    { id: "notifications", title: unread ? `${t.notifications} · ${unread}` : t.notifications },
    { id: "safety", title: t.safety, hint: t.safetyHint },
    { id: "support", title: t.support },
    { id: "settings", title: t.settings },
  ];

  useEffect(() => {
    login("rider")
      .then(async (token) => {
        setTokens((prev) => ({ ...prev, rider: token }));
        setScreenToken(token);
        setQuoteCents(await quoteFare());
        const notes = await req("/notifications/unread", { token }).catch(() => ({ notifications: [] }));
        setUnread((notes.notifications || []).length);
      })
      .catch((e) => setErr(String(e.message || e)));
  }, []);

  useEffect(() => {
    const id = setInterval(() => {
      if (role !== "driver" || !tokens.driver || !online || onTrip) return;
      req("/drivers/offers", { token: tokens.driver })
        .then((data) => {
          const offer = (data.offers || [])[0];
          if (!offer) return;
          const nextId = offer.RideID || offer.rideId || offer.ride_id;
          setOfferRideId(nextId);
          setRideId(nextId || rideId);
          setHasOffer(true);
        })
        .catch(() => {});
    }, 2000);
    return () => clearInterval(id);
  }, [role, tokens.driver, online, onTrip, rideId]);

  useEffect(() => {
    const token = role === "driver" ? tokens.driver : tokens.rider;
    const watching = role === "driver" ? onTrip : searching;
    if (!rideId || !token || !watching) return;
    let alive = true;
    const poll = () =>
      req(`/rides/${rideId}`, { token, lang })
        .then((data) => {
          if (!alive) return;
          const ride = data.ride || {};
          setRideStatus(ride.status || "");
          setVehicle(ride.vehicle || null);
          setCancelReason(ride.cancellationReason || "");
        })
        .catch(() => {});
    poll();
    const id = setInterval(poll, 3000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [role, rideId, searching, onTrip, tokens.rider, tokens.driver, lang]);

  useEffect(() => {
    if (role !== "rider" || rideStatus !== "matched" || tripPin || !rideId || !tokens.rider) return;
    req(`/rides/${rideId}/trip-pin`, { method: "POST", token: tokens.rider, lang })
      .then((data) => setTripPin(String(data.tripPin || "")))
      .catch(() => {});
  }, [role, rideStatus, tripPin, rideId, tokens.rider, lang]);

  useEffect(() => {
    if (role === "driver" && onTrip && rideStatus === "cancelled") {
      setOnTrip(false);
      setTripStep("arrive");
      setErr(t.riderCancelled);
    }
  }, [role, onTrip, rideStatus, t]);

  async function goOnline() {
    try {
      setErr("");
      const token = tokens.driver || (await login("driver"));
      setTokens((prev) => ({ ...prev, driver: token }));
      await req("/drivers/location", {
        method: "POST",
        token,
        body: { lat: FROM.lat, lng: FROM.lng, heading: 90 },
      });
      await req("/drivers/availability", { method: "PUT", token, body: { online: true } });
      setOnline(true);
    } catch (e: any) {
      setErr(e.message);
    }
  }

  async function requestRide() {
    try {
      setErr("");
      const token = tokens.rider || (await login("rider"));
      setTokens((prev) => ({ ...prev, rider: token }));
      const data = await req("/rides", {
        method: "POST",
        token,
        body: {
          pickupLat: FROM.lat,
          pickupLng: FROM.lng,
          pickupAddress: pickup,
          destinationLat: TO.lat,
          destinationLng: TO.lng,
          destinationAddress: dropoff,
          suggestedFareCents: fare,
          promoCode: promoCode || undefined,
          businessAccountId: businessId || undefined,
        },
      });
      const id = data.ride.id;
      setRideId(id);
      await req("/rides/" + id + "/payment-method", {
        method: "PUT",
        token,
        body: { method: payment },
      }).catch(() => {});
      setRideStatus(data.ride.status || "requested");
      setVehicle(null);
      setCancelReason("");
      setTripPin("");
      setRating(5);
      setRated(false);
      setSearching(true);
    } catch (e: any) {
      setErr(e.message);
    }
  }

  async function acceptOffer() {
    try {
      setErr("");
      const data = await req("/rides/" + (offerRideId || rideId) + "/accept", {
        method: "POST",
        token: tokens.driver,
      });
      setRideId(data.ride.id);
      setRideStatus(data.ride.status || "matched");
      setHasOffer(false);
      setOnTrip(true);
      setTripStep("arrive");
      setPinInput("");
      setRating(5);
      setRated(false);
    } catch (e: any) {
      setErr(e.message);
    }
  }

  async function declineOffer() {
    const id = offerRideId || rideId;
    setHasOffer(false);
    if (!id) return;
    await req(`/rides/${id}/decline`, { method: "POST", token: tokens.driver, lang }).catch((e) =>
      setErr(e.message),
    );
    setOfferRideId("");
  }

  async function reportNoShow() {
    try {
      setErr("");
      await req(`/rides/${rideId}/no-show`, { method: "POST", token: tokens.driver, lang });
      setOnTrip(false);
      setTripStep("arrive");
      setErr(t.riderNoShow);
    } catch (e: any) {
      setErr(e.message);
    }
  }

  function triggerSOS() {
    const token = role === "driver" ? tokens.driver : tokens.rider;
    Alert.alert(t.sos, t.sosConfirm, [
      { text: t.cancel, style: "cancel" },
      {
        text: t.sos,
        style: "destructive",
        onPress: () =>
          req(`/rides/${rideId}/sos`, { method: "POST", token, lang, body: { note: role } })
            .then(() => Alert.alert(t.sos, t.sosSent))
            .catch((e) => setErr(e.message)),
      },
    ]);
  }

  async function submitRating() {
    try {
      setErr("");
      const token = role === "driver" ? tokens.driver : tokens.rider;
      await req(`/rides/${rideId}/reviews`, { method: "POST", token, lang, body: { rating } });
      setRated(true);
    } catch (e: any) {
      setErr(e.message);
    }
  }

  async function advanceTrip() {
    try {
      setErr("");
      if (tripStep === "arrive") {
        await req("/rides/" + rideId + "/arrive", { method: "POST", token: tokens.driver });
        setTripStep("start");
        return;
      }
      if (tripStep === "start") {
        if (pinInput) {
          await req(`/rides/${rideId}/trip-pin/verify`, {
            method: "POST",
            token: tokens.driver,
            lang,
            body: { pin: pinInput },
          });
        }
        await req("/rides/" + rideId + "/start", { method: "POST", token: tokens.driver });
        setTripStep("complete");
        return;
      }
      await req("/rides/" + rideId + "/complete", { method: "POST", token: tokens.driver });
      await req("/rides/" + rideId + "/cash-payment", {
        method: "POST",
        token: tokens.driver,
        body: { amountPaid: fare },
      }).catch(() => {});
      setTripStep("done");
    } catch (e: any) {
      setErr(e.message);
    }
  }

  async function cancelRide() {
    if (rideId && riderCanCancel) {
      await req("/rides/" + rideId + "/cancel", {
        method: "POST",
        token: tokens.rider,
        body: { reason: "rider_cancelled" },
      }).catch(() => {});
    }
    setSearching(false);
    setRideStatus("");
    setTripPin("");
  }

  function switchRole(next: Role) {
    setRole(next);
    setMenuOpen(false);
    setScreen(null);
    if (next === "driver") goOnline();
  }

  async function openScreen(id: ScreenId) {
    try {
      setErr("");
      const tok =
        role === "driver" ? tokens.driver || (await login("driver")) : tokens.rider || (await login("rider"));
      setTokens((prev) => (role === "driver" ? { ...prev, driver: tok } : { ...prev, rider: tok }));
      setScreenToken(tok);
      setMenuOpen(false);
      setScreen(id);
    } catch (e: any) {
      setErr(e.message);
    }
  }

  return (
    <SafeAreaView style={styles.safe}>
      <StatusBar style="dark" />
      <View style={styles.map}>
        <Text style={styles.city}>Baixa</Text>
        <View style={[styles.pin, styles.pickup]} />
        <View style={[styles.pin, styles.drop]} />
        <Text style={styles.talatona}>Talatona</Text>
      </View>

      <Pressable style={styles.menuBtn} onPress={() => setMenuOpen(true)} accessibilityLabel={t.menu}>
        <Text style={styles.menuIcon}>☰</Text>
      </Pressable>
      <View style={styles.switchRow}>
        <Pressable style={[styles.chip, role === "rider" && styles.chipOn]} onPress={() => setRole("rider")}>
          <Text style={[styles.chipText, role === "rider" && styles.chipTextOn]}>{t.rider}</Text>
        </Pressable>
        <Pressable
          style={[styles.chip, role === "driver" && styles.chipOn]}
          onPress={() => switchRole("driver")}
        >
          <Text style={[styles.chipText, role === "driver" && styles.chipTextOn]}>{t.driver}</Text>
        </Pressable>
      </View>

      <View style={styles.sheet}>
        <View style={styles.grab} />
        <ScrollView contentContainerStyle={styles.sheetBody}>
          {role === "rider" && !searching && (
            <>
              <Text style={styles.h1}>{t.whereTo}</Text>
              <View style={styles.stops}>
                <TextInput style={styles.input} value={pickup} onChangeText={setPickup} />
                <TextInput style={styles.input} value={dropoff} onChangeText={setDropoff} />
              </View>
              <Text style={styles.h2}>{t.yourOffer}</Text>
              <View style={styles.bid}>
                <Pressable onPress={() => setQuoteCents((n) => Math.max(100, n - 100))} style={styles.step}>
                  <Text style={styles.stepText}>−</Text>
                </Pressable>
                <Text style={styles.fare}>{kz(quoteCents)}</Text>
                <Pressable onPress={() => setQuoteCents((n) => n + 100)} style={styles.step}>
                  <Text style={styles.stepText}>+</Text>
                </Pressable>
              </View>
              <Pressable
                style={[styles.product, product === "go" && styles.productOn]}
                onPress={() => setProduct("go")}
              >
                <Text style={styles.productTitle}>RideX Go</Text>
                <Text>{kz(quoteCents)}</Text>
              </Pressable>
              <Pressable
                style={[styles.product, product === "comfort" && styles.productOn]}
                onPress={() => setProduct("comfort")}
              >
                <Text style={styles.productTitle}>Comfort</Text>
                <Text>{kz(Math.round(quoteCents * 1.25))}</Text>
              </Pressable>
              <Text style={styles.pay}>
                {payLabel}
                {promoCode ? ` · ${promoCode}` : ""}
              </Text>
              <Pressable style={styles.cta} onPress={requestRide}>
                <Text style={styles.ctaText}>
                  {t.request} {product === "go" ? "RideX Go" : "Comfort"}
                </Text>
              </Pressable>
            </>
          )}

          {role === "rider" && searching && (
            <>
              <Text style={styles.pill}>{riderStatusLabel}</Text>
              {rideStatus === "requested" || !rideStatus ? (
                <Text style={styles.h1}>{t.connecting}</Text>
              ) : rideStatus === "cancelled" ? (
                <Text style={styles.meta}>
                  {cancelReason === "rider_no_show" ? t.riderNoShow : t.rideCancelled}
                </Text>
              ) : (
                <>
                  <Text style={styles.driver}>{vehicleText}</Text>
                  <Text style={styles.meta}>
                    {rideStatus === "completed" ? t.thanks : riderStatusLabel} · {kz(fare)}
                  </Text>
                </>
              )}
              {!!tripPin && (rideStatus === "matched" || rideStatus === "driver_arriving") && (
                <View style={styles.pinBox}>
                  <Text style={styles.h2}>{t.yourPin}</Text>
                  <Text style={styles.pinCode}>{tripPin}</Text>
                  <Text style={styles.meta}>{t.pinShare}</Text>
                </View>
              )}
              {rideStatus === "completed" && (
                <RatingRow
                  title={t.rateDriver}
                  rating={rating}
                  rated={rated}
                  t={t}
                  onRate={setRating}
                  onSubmit={submitRating}
                />
              )}
              <View style={styles.row}>
                {rideActive && (
                  <Pressable style={styles.sos} onPress={triggerSOS}>
                    <Text style={styles.ctaText}>{t.sos}</Text>
                  </Pressable>
                )}
                {(riderCanCancel || rideFinished) && (
                  <Pressable style={styles.ghost} onPress={cancelRide}>
                    <Text style={styles.ghostText}>{rideFinished ? t.newTrip : t.cancel}</Text>
                  </Pressable>
                )}
              </View>
            </>
          )}

          {role === "driver" && !hasOffer && !onTrip && (
            <>
              <Text style={styles.h1}>{online ? t.online : t.offline}</Text>
              <Text style={styles.meta}>{online ? t.waiting : t.tapGo}</Text>
              <Pressable style={[styles.cta, online && styles.ctaGreen]} onPress={goOnline}>
                <Text style={styles.ctaText}>{online ? "ON" : "GO"}</Text>
              </Pressable>
            </>
          )}

          {role === "driver" && hasOffer && !onTrip && (
            <>
              <Text style={styles.h1}>{t.newRequest}</Text>
              <Text style={styles.meta}>{pickup}</Text>
              <Text style={styles.meta}>{dropoff}</Text>
              <Text style={styles.fare}>{kz(fare)}</Text>
              <View style={styles.row}>
                <Pressable style={styles.ghost} onPress={declineOffer}>
                  <Text style={styles.ghostText}>{t.decline}</Text>
                </Pressable>
                <Pressable style={[styles.cta, styles.ctaGreen]} onPress={acceptOffer}>
                  <Text style={styles.ctaText}>{t.accept}</Text>
                </Pressable>
              </View>
            </>
          )}

          {role === "driver" && onTrip && (
            <>
              <Text style={styles.pill}>
                {tripStep === "arrive"
                  ? t.enRoute
                  : tripStep === "start"
                    ? t.atPickup
                    : tripStep === "complete"
                      ? t.inTrip
                      : t.done}
              </Text>
              <Text style={styles.driver}>{t.rider}</Text>
              <Text style={styles.meta}>{tripStep === "complete" ? dropoff : pickup}</Text>
              {tripStep === "start" && (
                <View style={styles.pinBox}>
                  <Text style={styles.h2}>{t.riderPin}</Text>
                  <TextInput
                    style={styles.pinInput}
                    value={pinInput}
                    onChangeText={(v) => setPinInput(v.replace(/\D/g, "").slice(0, 4))}
                    keyboardType="number-pad"
                    maxLength={4}
                    placeholder="••••"
                  />
                  <Text style={styles.meta}>{t.pinHint}</Text>
                </View>
              )}
              {tripStep === "done" && (
                <RatingRow
                  title={t.rateRider}
                  rating={rating}
                  rated={rated}
                  t={t}
                  onRate={setRating}
                  onSubmit={submitRating}
                />
              )}
              <View style={styles.row}>
                {tripStep !== "done" && (
                  <Pressable style={styles.sos} onPress={triggerSOS}>
                    <Text style={styles.ctaText}>{t.sos}</Text>
                  </Pressable>
                )}
                <Pressable
                  style={styles.cta}
                  onPress={() => {
                    if (tripStep === "done") {
                      setOnTrip(false);
                      return;
                    }
                    advanceTrip();
                  }}
                >
                  <Text style={styles.ctaText}>
                    {tripStep === "arrive"
                      ? t.arrived
                      : tripStep === "start"
                        ? t.startTrip
                        : tripStep === "complete"
                          ? t.complete
                          : t.readyNext}
                  </Text>
                </Pressable>
              </View>
              {tripStep === "start" && (
                <Pressable style={styles.ghost} onPress={reportNoShow}>
                  <Text style={styles.ghostText}>{t.noShow}</Text>
                </Pressable>
              )}
            </>
          )}

          {!!err && <Text style={styles.err}>{err}</Text>}
        </ScrollView>
      </View>

      <Modal visible={menuOpen} animationType="fade" transparent onRequestClose={() => setMenuOpen(false)}>
        <View style={styles.drawerWrap}>
          <SafeAreaView style={styles.drawer}>
            <ScrollView contentContainerStyle={styles.drawerBody}>
              <Pressable style={styles.profile} onPress={() => openScreen("settings")}>
                <View style={styles.avatar}>
                  <Text style={styles.avatarText}>{accountName.slice(0, 1)}</Text>
                </View>
                <View>
                  <Text style={styles.profileName}>{accountName}</Text>
                  <Text style={styles.profileMeta}>{role === "driver" ? t.driver : t.rider} · 4.93</Text>
                </View>
              </Pressable>
            <Text style={styles.section}>{t.language}</Text>
            <View style={styles.langRow}>
              <Pressable
                style={[styles.langChip, lang === "pt" && styles.langOn]}
                onPress={() => setLang("pt")}
              >
                <Text style={[styles.langText, lang === "pt" && styles.langTextOn]}>{t.portuguese}</Text>
              </Pressable>
              <Pressable
                style={[styles.langChip, lang === "en" && styles.langOn]}
                onPress={() => setLang("en")}
              >
                <Text style={[styles.langText, lang === "en" && styles.langTextOn]}>{t.english}</Text>
              </Pressable>
            </View>
            {(role === "rider" ? riderItems : driverItems).map((item) => (
              <Pressable key={item.id} style={styles.item} onPress={() => openScreen(item.id)}>
                <Text style={styles.itemText}>{item.title}</Text>
                {item.hint ? <Text style={styles.itemHint}>{item.hint}</Text> : null}
              </Pressable>
            ))}
            <Pressable
              style={styles.item}
              onPress={() => switchRole(role === "rider" ? "driver" : "rider")}
            >
              <Text style={styles.itemText}>{role === "rider" ? t.switchToDriver : t.switchToRider}</Text>
            </Pressable>
              <Pressable style={styles.close} onPress={() => setMenuOpen(false)}>
                <Text style={styles.closeText}>{t.close}</Text>
              </Pressable>
            </ScrollView>
          </SafeAreaView>
          <Pressable style={styles.scrim} onPress={() => setMenuOpen(false)} />
        </View>
      </Modal>

      <Modal visible={!!screen && !!screenToken} animationType="slide" onRequestClose={() => setScreen(null)}>
        {screen && screenToken ? (
          <AccountScreen
            screen={screen}
            token={screenToken}
            role={role}
            lang={lang}
            t={t}
            payment={payment}
            promoCode={promoCode}
            businessId={businessId}
            lastRideId={rideId}
            onBack={() => setScreen(null)}
            onPayment={setPayment}
            onPromo={setPromoCode}
            onBusiness={setBusinessId}
            onPlace={(address, asPickup) => {
              if (asPickup) setPickup(address);
              else setDropoff(address);
            }}
          />
        ) : null}
      </Modal>
    </SafeAreaView>
  );
}

function RatingRow(props: {
  title: string;
  rating: number;
  rated: boolean;
  t: Copy;
  onRate: (n: number) => void;
  onSubmit: () => void;
}) {
  if (props.rated) return <Text style={styles.meta}>{props.t.rated}</Text>;
  return (
    <View style={styles.pinBox}>
      <Text style={styles.h2}>{props.title}</Text>
      <View style={styles.stars}>
        {[1, 2, 3, 4, 5].map((n) => (
          <Pressable key={n} onPress={() => props.onRate(n)} accessibilityLabel={`${n}`}>
            <Text style={[styles.star, n <= props.rating && styles.starOn]}>★</Text>
          </Pressable>
        ))}
      </View>
      <Pressable style={styles.cta} onPress={props.onSubmit}>
        <Text style={styles.ctaText}>{props.t.sendRating}</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: "#d7e0da" },
  map: { flex: 1, backgroundColor: "#d7e0da" },
  city: { position: "absolute", left: 48, top: 90, color: "#7a847e", fontWeight: "700", fontSize: 22 },
  talatona: { position: "absolute", right: 36, bottom: 220, color: "#7a847e", fontWeight: "700", fontSize: 22 },
  pin: {
    position: "absolute",
    width: 16,
    height: 16,
    borderRadius: 8,
    borderWidth: 2,
    borderColor: "#fff",
  },
  pickup: { left: "34%", top: "42%", backgroundColor: "#111" },
  drop: { left: "62%", top: "58%", backgroundColor: "#06c167" },
  menuBtn: {
    position: "absolute",
    top: 54,
    left: 16,
    width: 44,
    height: 44,
    borderRadius: 22,
    backgroundColor: "#fff",
    alignItems: "center",
    justifyContent: "center",
    shadowColor: "#000",
    shadowOpacity: 0.12,
    shadowRadius: 12,
  },
  menuIcon: { fontSize: 20, fontWeight: "800" },
  switchRow: {
    position: "absolute",
    top: 54,
    alignSelf: "center",
    flexDirection: "row",
    backgroundColor: "#fff",
    borderRadius: 999,
    padding: 4,
    gap: 4,
  },
  chip: { paddingHorizontal: 14, paddingVertical: 8, borderRadius: 999 },
  chipOn: { backgroundColor: "#111" },
  chipText: { fontWeight: "700", color: "#6b7280" },
  chipTextOn: { color: "#fff" },
  sheet: {
    backgroundColor: "#fff",
    borderTopLeftRadius: 28,
    borderTopRightRadius: 28,
    maxHeight: "58%",
    paddingHorizontal: 18,
    paddingBottom: 18,
  },
  grab: {
    width: 42,
    height: 5,
    backgroundColor: "#e5e7eb",
    borderRadius: 99,
    alignSelf: "center",
    marginVertical: 8,
  },
  sheetBody: { paddingBottom: 12 },
  h1: { fontSize: 26, fontWeight: "800", marginBottom: 12 },
  h2: { fontSize: 12, fontWeight: "700", color: "#6b7280", letterSpacing: 0.6, marginBottom: 8 },
  stops: { backgroundColor: "#f4f5f7", borderRadius: 18, padding: 12, marginBottom: 12, gap: 10 },
  input: { fontSize: 16, fontWeight: "600" },
  bid: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", marginBottom: 12 },
  step: { width: 40, height: 40, borderRadius: 20, backgroundColor: "#f3f4f6", alignItems: "center", justifyContent: "center" },
  stepText: { fontSize: 22, fontWeight: "700" },
  fare: { fontSize: 22, fontWeight: "800" },
  product: { flexDirection: "row", justifyContent: "space-between", padding: 12, borderRadius: 16, borderWidth: 2, borderColor: "transparent" },
  productOn: { borderColor: "#111", backgroundColor: "#fafafa" },
  productTitle: { fontWeight: "800" },
  pay: { marginVertical: 8, fontWeight: "700" },
  cta: { backgroundColor: "#111", borderRadius: 16, padding: 16, alignItems: "center", marginTop: 8, flex: 1 },
  ctaGreen: { backgroundColor: "#06c167" },
  ctaText: { color: "#fff", fontWeight: "800" },
  ghost: { backgroundColor: "#f3f4f6", borderRadius: 16, padding: 16, alignItems: "center", marginTop: 8, flex: 1 },
  ghostText: { fontWeight: "800" },
  sos: { backgroundColor: "#dc2626", borderRadius: 16, padding: 16, alignItems: "center", marginTop: 8, minWidth: 72 },
  pinBox: { backgroundColor: "#f4f5f7", borderRadius: 18, padding: 14, marginTop: 12 },
  pinCode: { fontSize: 34, fontWeight: "800", letterSpacing: 8 },
  pinInput: { fontSize: 28, fontWeight: "800", letterSpacing: 8, paddingVertical: 4 },
  stars: { flexDirection: "row", gap: 8, marginVertical: 6 },
  star: { fontSize: 32, color: "#d1d5db" },
  starOn: { color: "#f59e0b" },
  row: { flexDirection: "row", gap: 8 },
  pill: {
    alignSelf: "flex-start",
    backgroundColor: "#111",
    color: "#fff",
    overflow: "hidden",
    borderRadius: 999,
    paddingHorizontal: 10,
    paddingVertical: 6,
    fontWeight: "700",
    marginBottom: 10,
  },
  driver: { fontWeight: "800", fontSize: 16 },
  meta: { color: "#6b7280", marginTop: 4 },
  err: { color: "#b91c1c", marginTop: 8 },
  drawerWrap: { flex: 1, flexDirection: "row" },
  scrim: { flex: 1, backgroundColor: "rgba(0,0,0,0.35)" },
  drawer: { width: 300, backgroundColor: "#fff" },
  drawerBody: { paddingHorizontal: 18, paddingTop: 12, paddingBottom: 24 },
  profile: { flexDirection: "row", alignItems: "center", gap: 12, marginBottom: 18, marginTop: 8 },
  avatar: { width: 52, height: 52, borderRadius: 26, backgroundColor: "#111", alignItems: "center", justifyContent: "center" },
  avatarText: { color: "#fff", fontWeight: "800", fontSize: 20 },
  profileName: { fontWeight: "800", fontSize: 18 },
  profileMeta: { color: "#6b7280", marginTop: 2 },
  section: { fontSize: 12, fontWeight: "700", color: "#6b7280", letterSpacing: 0.6, marginBottom: 8 },
  langRow: { flexDirection: "row", gap: 8, marginBottom: 12 },
  langChip: { flex: 1, borderRadius: 999, borderWidth: 1.5, borderColor: "#ececec", paddingVertical: 10, alignItems: "center" },
  langOn: { backgroundColor: "#111", borderColor: "#111" },
  langText: { fontWeight: "700", color: "#111" },
  langTextOn: { color: "#fff" },
  item: { paddingVertical: 14, borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: "#ececec" },
  itemText: { fontSize: 16, fontWeight: "600" },
  itemHint: { fontSize: 12, color: "#6b7280", marginTop: 2 },
  close: { marginTop: 16, marginBottom: 20, backgroundColor: "#f3f4f6", borderRadius: 14, padding: 14, alignItems: "center" },
  closeText: { fontWeight: "800" },
});
