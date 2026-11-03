import { useEffect, useState } from "react";
import { api, type UsageSummary } from "../api";
import { formatTokens, money, useCurrency } from "../currency";

/** What a task's sessions have cost so far. Quiet when it can't tell. */
export function TaskCost({ taskId, sessions }: { taskId: number; sessions: number }) {
  const [currency] = useCurrency();
  const [s, setS] = useState<UsageSummary | null>(null);
  useEffect(() => {
    let live = true;
    setS(null);
    api.usage(0, taskId).then(
      (v) => live && setS(v),
      () => {},
    );
    return () => {
      live = false;
    };
    // Sessions joining the task change its cost.
  }, [taskId, sessions]);
  if (!s || s.total.calls === 0) return null;
  const m = money(s.fx, currency);
  const unpriced = (s.total.unpriced_calls ?? 0) > 0;
  return (
    <p className="task-cost muted" title="Priced at live model rates from the sessions' transcripts">
      <span className="task-cost-value">{m.format(s.total.cost_usd)}</span>
      {unpriced && "+"} in Claude usage · {formatTokens(s.total.total_tokens)} tokens
    </p>
  );
}
