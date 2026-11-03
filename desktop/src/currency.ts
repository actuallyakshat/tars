// Costs arrive in USD; this shows them in the currency the user picked, at
// the live rate the CLI fetched. Without a rate, costs stay in USD.
import { useCallback, useEffect, useState } from "react";
import type { UsageSummary } from "./api";

export type Currency = "USD" | "INR";
export const CURRENCIES: Currency[] = ["USD", "INR"];

const KEY = "tars.currency";
const EVENT = "tars-currency";

function stored(): Currency {
  const v = localStorage.getItem(KEY);
  return v === "INR" ? "INR" : "USD";
}

/** The picked currency, shared by every view and remembered across launches. */
export function useCurrency(): [Currency, (c: Currency) => void] {
  const [c, setC] = useState<Currency>(stored);
  useEffect(() => {
    const h = () => setC(stored());
    window.addEventListener(EVENT, h);
    return () => window.removeEventListener(EVENT, h);
  }, []);
  const set = useCallback((next: Currency) => {
    localStorage.setItem(KEY, next);
    window.dispatchEvent(new Event(EVENT));
  }, []);
  return [c, set];
}

/** The rate for c, or null when there's none (USD is always 1). */
export function rateFor(fx: UsageSummary["fx"] | undefined, c: Currency): number | null {
  if (c === "USD") return 1;
  const r = fx?.rates?.[c];
  return typeof r === "number" && r > 0 ? r : null;
}

const formats = new Map<string, Intl.NumberFormat>();
function fmt(c: Currency, digits: number) {
  const k = `${c}${digits}`;
  let f = formats.get(k);
  if (!f) {
    // en-IN groups rupees in lakhs and crores (₹1,23,456).
    f = new Intl.NumberFormat(c === "INR" ? "en-IN" : "en-US", {
      style: "currency",
      currency: c,
      minimumFractionDigits: digits,
      maximumFractionDigits: digits,
    });
    formats.set(k, f);
  }
  return f;
}

/** Formats an amount already in c. Tiny amounts get more digits so they don't read as zero. */
export function formatMoney(amount: number, c: Currency): string {
  const abs = Math.abs(amount);
  const digits = abs > 0 && abs < 0.1 ? (c === "INR" ? 2 : 3) : abs >= 10_000 ? 0 : 2;
  return fmt(c, digits).format(amount);
}

/** A converter for one summary: USD → the picked currency, falling back to USD. */
export function money(fx: UsageSummary["fx"] | undefined, c: Currency) {
  const rate = rateFor(fx, c);
  const cur: Currency = rate === null ? "USD" : c;
  const r = rate ?? 1;
  return { currency: cur, format: (usd: number) => formatMoney(usd * r, cur) };
}

const compact = new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 });
export const formatTokens = (n: number) => compact.format(n);
