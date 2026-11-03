import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, type LiveStatus, type UsageLine, type UsageSummary } from "../api";
import { CURRENCIES, formatTokens, money, rateFor, useCurrency, type Currency } from "../currency";
import { Page } from "./Pages";

export const PERIODS = [
  { days: 7, label: "7 days" },
  { days: 30, label: "30 days" },
  { days: 90, label: "90 days" },
  { days: 0, label: "All time" },
] as const;

const PERIOD_KEY = "tars.usage.days";

const STATUS_WORD: Record<LiveStatus, string> = {
  fresh: "just fetched",
  cached: "cached",
  stale: "offline, last known",
  unavailable: "unavailable",
};

type Fmt = (usd: number) => string;

function storedPeriod(): number {
  const v = Number(localStorage.getItem(PERIOD_KEY));
  return PERIODS.some((p) => p.days === v) && localStorage.getItem(PERIOD_KEY) !== null ? v : 30;
}

function Segmented<T extends string | number>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: { value: T; label: string; disabled?: boolean; title?: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div className="segmented" role="radiogroup" aria-label={label}>
      {options.map((o) => (
        <button
          key={String(o.value)}
          role="radio"
          aria-checked={o.value === value}
          className={o.value === value ? "on" : ""}
          disabled={o.disabled}
          title={o.title}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

function Tile({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="usage-tile">
      <div className="usage-tile-label">{label}</div>
      <div className="usage-tile-value">{value}</div>
      {sub && <div className="usage-tile-sub muted">{sub}</div>}
    </div>
  );
}

const dayLabel = (key: string, opts: Intl.DateTimeFormatOptions = { month: "short", day: "numeric" }) =>
  new Date(`${key}T00:00:00`).toLocaleDateString(undefined, opts);

/** Cost per day as bars, one hue; hovering or focusing a bar shows its numbers. */
function DailyChart({ days, fmt }: { days: UsageLine[]; fmt: Fmt }) {
  const [hover, setHover] = useState<number | null>(null);
  const max = Math.max(...days.map((d) => d.cost_usd), 0);
  if (!days.length) return null;
  const W = 720;
  const H = 160;
  const gap = days.length > 60 ? 1 : 2;
  const bw = Math.max(1, (W - gap * (days.length - 1)) / days.length);
  const h = hover === null ? null : days[hover];
  const ticks = [0, Math.floor((days.length - 1) / 2), days.length - 1].filter((v, i, a) => a.indexOf(v) === i);
  return (
    <figure className="usage-chart" aria-label="Cost per day">
      <div className="usage-chart-head">
        <span className="muted">Cost per day</span>
        <span className="usage-chart-max muted">{max > 0 ? `peak ${fmt(max)}` : "no spend"}</span>
      </div>
      <div className="usage-chart-plot" onMouseLeave={() => setHover(null)}>
        <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" role="img" aria-label="Bar chart of cost per day">
          <line x1="0" x2={W} y1={H - 0.5} y2={H - 0.5} className="usage-chart-base" />
          {days.map((d, i) => {
            const bh = max > 0 ? (d.cost_usd / max) * (H - 6) : 0;
            const x = i * (bw + gap);
            return (
              <g
                key={d.key}
                tabIndex={0}
                aria-label={`${d.key}: ${fmt(d.cost_usd)}`}
                onMouseEnter={() => setHover(i)}
                onFocus={() => setHover(i)}
                onBlur={() => setHover(null)}
              >
                {/* The hit target is the whole column, wider than the bar. */}
                <rect x={x} y={0} width={bw + gap} height={H} fill="transparent" />
                {bh > 0 && (
                  <rect
                    x={x}
                    y={H - Math.max(bh, 2)}
                    width={bw}
                    height={Math.max(bh, 2)}
                    rx={Math.min(3, bw / 2)}
                    className={"usage-bar" + (hover === i ? " on" : "")}
                  />
                )}
              </g>
            );
          })}
        </svg>
        {h && (
          <div
            className="usage-tip"
            role="status"
            style={{ left: `${((hover! + 0.5) / days.length) * 100}%` }}
          >
            <strong>{dayLabel(h.key, { weekday: "short", month: "short", day: "numeric" })}</strong>
            <span>{fmt(h.cost_usd)}</span>
            <span className="muted">
              {formatTokens(h.total_tokens)} tokens · {h.sessions} {h.sessions === 1 ? "session" : "sessions"}
            </span>
          </div>
        )}
      </div>
      <div className="usage-chart-axis muted">
        {ticks.map((i) => (
          <span key={i} style={{ left: `${((i + 0.5) / days.length) * 100}%` }}>
            {dayLabel(days[i].key)}
          </span>
        ))}
      </div>
    </figure>
  );
}

function ShareBar({ share }: { share: number }) {
  return (
    <span className="usage-share" aria-hidden="true">
      <span style={{ width: `${Math.max(0, Math.min(1, share)) * 100}%` }} />
    </span>
  );
}

function Breakdown({
  title,
  rows,
  total,
  fmt,
  empty,
  name,
  onOpen,
  limit = 8,
}: {
  title: string;
  rows: UsageLine[];
  total: number;
  fmt: Fmt;
  empty: string;
  name: (l: UsageLine) => string;
  onOpen?: (l: UsageLine) => void;
  limit?: number;
}) {
  const [all, setAll] = useState(false);
  const shown = all ? rows : rows.slice(0, limit);
  return (
    <section className="usage-card" aria-label={title}>
      <h2 className="usage-card-title">{title}</h2>
      {rows.length === 0 ? (
        <p className="muted usage-empty">{empty}</p>
      ) : (
        <table className="usage-table">
          <thead>
            <tr>
              <th scope="col">Name</th>
              <th scope="col" className="num">Cost</th>
              <th scope="col" className="num">Tokens</th>
              <th scope="col" className="num">Sessions</th>
            </tr>
          </thead>
          <tbody>
            {shown.map((l) => (
              <tr key={l.key}>
                <th scope="row">
                  {onOpen ? (
                    <button className="link usage-open" onClick={() => onOpen(l)} title="Open the task">
                      {name(l)}
                    </button>
                  ) : (
                    <span className={l.unpriced_calls === l.calls ? "muted" : ""}>{name(l)}</span>
                  )}
                  <ShareBar share={total > 0 ? l.cost_usd / total : 0} />
                </th>
                <td className="num">{l.unpriced_calls === l.calls ? <span className="muted" title="No price for this model">—</span> : fmt(l.cost_usd)}</td>
                <td className="num muted">{formatTokens(l.total_tokens)}</td>
                <td className="num muted">{l.sessions}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {rows.length > limit && (
        <button className="link usage-more" onClick={() => setAll(!all)}>
          {all ? "Show fewer" : `Show all ${rows.length}`}
        </button>
      )}
    </section>
  );
}

function CostByType({ line, fmt }: { line: UsageLine; fmt: Fmt }) {
  const rows = [
    ["Output", line.cost.output, line.tokens.output],
    ["Cache writes", line.cost.cache_write, line.tokens.cache_write],
    ["Cache reads", line.cost.cache_read, line.tokens.cache_read],
    ["Uncached input", line.cost.input, line.tokens.input],
  ] as const;
  const total = rows.reduce((s, r) => s + r[1], 0);
  return (
    <section className="usage-card" aria-label="Cost by token type">
      <h2 className="usage-card-title">By token type</h2>
      <table className="usage-table">
        <tbody>
          {rows.map(([label, cost, tokens]) => (
            <tr key={label}>
              <th scope="row">
                {label}
                <ShareBar share={total > 0 ? cost / total : 0} />
              </th>
              <td className="num">{fmt(cost)}</td>
              <td className="num muted">{formatTokens(tokens)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function Sources({ s, currency }: { s: UsageSummary; currency: Currency }) {
  const inr = s.fx.rates.INR;
  const fetched = (at?: string) => (at ? new Date(at).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }) : "");
  return (
    <footer className="usage-sources muted">
      {inr ? (
        <p>
          1 USD = ₹{inr.toFixed(2)} · {s.fx.source} · rates for {s.fx.date} · {STATUS_WORD[s.fx.status]}
        </p>
      ) : (
        <p>
          Exchange rates couldn't be fetched{currency !== "USD" ? ", so costs are shown in USD" : ""}. They'll load when you're back online.
        </p>
      )}
      {s.pricing.status === "unavailable" ? (
        <p>Model prices couldn't be fetched, so costs show as zero. Check your connection and refresh.</p>
      ) : (
        <p>
          Prices from LiteLLM's public table ({s.pricing.models.toLocaleString()} models) · fetched {fetched(s.pricing.fetched_at)} · {STATUS_WORD[s.pricing.status]}. Costs are list
          prices for the tokens used; a subscription plan bills differently.
        </p>
      )}
      {s.unpriced_models.length > 0 && <p>No price yet for {s.unpriced_models.join(", ")}; their tokens are counted but not their cost.</p>}
      <p>
        Read {s.files.toLocaleString()} transcripts in {(s.scan_ms / 1000).toFixed(1)}s.
      </p>
    </footer>
  );
}

/** Placeholders shaped like the page, while the first read runs. */
function UsageSkeleton() {
  return (
    <div className="usage" aria-busy="true" aria-label="Reading your Claude transcripts">
      <div className="usage-progress" role="progressbar" aria-label="Reading transcripts" />
      <div className="usage-tiles">
        {["Total cost", "Tokens", "Sessions", "Saved by caching"].map((l) => (
          <div key={l} className="usage-tile">
            <div className="usage-tile-label">{l}</div>
            <div className="skeleton skeleton-value" />
            <div className="skeleton skeleton-sub" />
          </div>
        ))}
      </div>
      <div className="usage-chart">
        <div className="usage-chart-head">
          <span className="muted">Reading your Claude transcripts…</span>
        </div>
        <div className="skeleton skeleton-chart" />
      </div>
    </div>
  );
}

export function UsagePage({ onBack, onOpenTask }: { onBack: () => void; onOpenTask: (id: number) => void }) {
  const [days, setDaysState] = useState(storedPeriod);
  const [currency, setCurrency] = useCurrency();
  const [data, setData] = useState<UsageSummary | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const seq = useRef(0);

  const load = useCallback((d: number) => {
    const n = ++seq.current;
    setLoading(true);
    setError(null);
    api.usage(d).then(
      (s) => {
        if (n !== seq.current) return;
        setData(s);
        setLoading(false);
      },
      (e: unknown) => {
        if (n !== seq.current) return;
        setError((e as { message?: string })?.message ?? String(e));
        setLoading(false);
      },
    );
  }, []);

  useEffect(() => load(days), [days, load]);

  const setDays = (d: number) => {
    localStorage.setItem(PERIOD_KEY, String(d));
    setDaysState(d);
  };

  const m = useMemo(() => money(data?.fx, currency), [data, currency]);
  const noRate = !!data && rateFor(data.fx, "INR") === null;
  const t = data?.total;
  const activeDays = data ? data.daily.filter((d) => d.calls > 0).length : 0;

  return (
    <Page title="Usage" onBack={onBack} full>
      <div className="usage-controls">
        <Segmented label="Period" value={days} options={PERIODS.map((p) => ({ value: p.days, label: p.label }))} onChange={setDays} />
        <Segmented<Currency>
          label="Currency"
          value={currency}
          options={CURRENCIES.map((c) => ({
            value: c,
            label: c === "INR" ? "₹ INR" : "$ USD",
            disabled: c === "INR" && noRate,
            title: c === "INR" && noRate ? "No exchange rate yet: tars couldn't fetch one" : undefined,
          }))}
          onChange={setCurrency}
        />
        <button className="ghost" onClick={() => load(days)} disabled={loading}>
          {loading ? "Reading…" : "Refresh"}
        </button>
      </div>
      <p className="page-lede muted">
        What your Claude Code sessions cost, read from their transcripts on this Mac and priced at today's model rates.
        {data && ` ${dayLabel(data.since, { month: "short", day: "numeric", year: "numeric" })} – ${dayLabel(data.until, { month: "short", day: "numeric", year: "numeric" })}.`}
      </p>

      {error && (
        <div className="usage-error" role="alert">
          <p>Couldn't read usage: {error}</p>
          <button className="ghost" onClick={() => load(days)}>
            Try again
          </button>
        </div>
      )}

      {!data && loading && <UsageSkeleton />}

      {data && t && (
        <div className={"usage" + (loading ? " usage-refreshing" : "")} aria-busy={loading}>
          {loading && <div className="usage-progress" role="progressbar" aria-label="Reading transcripts" />}
          <div className="usage-tiles">
            <Tile label="Total cost" value={m.format(t.cost_usd)} sub={activeDays ? `${m.format(t.cost_usd / activeDays)} per active day` : undefined} />
            <Tile
              label="Tokens"
              value={formatTokens(t.total_tokens)}
              sub={`${formatTokens(t.tokens.output)} output · ${formatTokens(t.tokens.cache_read)} cache reads`}
            />
            <Tile label="Sessions" value={t.sessions.toLocaleString()} sub={`${t.calls.toLocaleString()} model calls`} />
            <Tile label="Saved by caching" value={m.format(t.cache_savings_usd)} sub="vs. sending the same input uncached" />
          </div>

          {t.calls === 0 ? (
            <p className="muted usage-empty">No Claude Code activity in this period.</p>
          ) : (
            <>
              <DailyChart days={data.daily} fmt={m.format} />
              <div className="usage-grid">
                <Breakdown
                  title="By task"
                  rows={data.tasks}
                  total={t.cost_usd}
                  fmt={m.format}
                  empty="No sessions here are linked to a task yet."
                  name={(l) => `#${l.key} ${l.label ?? ""}`}
                  onOpen={(l) => onOpenTask(Number(l.key))}
                />
                <Breakdown title="By repo" rows={data.repos} total={t.cost_usd} fmt={m.format} empty="No repo information." name={(l) => l.key} />
                <Breakdown title="By model" rows={data.models} total={t.cost_usd} fmt={m.format} empty="No models." name={(l) => l.key} />
                <CostByType line={t} fmt={m.format} />
              </div>
            </>
          )}
          <Sources s={data} currency={currency} />
        </div>
      )}
    </Page>
  );
}
