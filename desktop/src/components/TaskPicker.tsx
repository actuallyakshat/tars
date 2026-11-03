import { useEffect, useId, useMemo, useRef, useState } from "react";
import type { Task } from "../api";
import { STATUS, matchesQuery } from "../model";

const MAX = 40;

// Picks one task: type to filter by title, #id, Jira, repo or tag; ↑↓ and
// Enter, or click. Open work comes first, most recently touched first.
export function TaskPicker({
  title,
  hint,
  tasks,
  exclude,
  prefer,
  actionLabel,
  onPick,
  onCancel,
}: {
  title: string;
  hint?: string;
  tasks: Task[];
  exclude?: number;
  /** Tasks in this repo are listed first. */
  prefer?: string;
  actionLabel: string;
  onPick: (id: number) => Promise<unknown>;
  onCancel: () => void;
}) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const [busy, setBusy] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  const listId = useId();
  useEffect(() => input.current?.focus(), []);

  const found = useMemo(() => {
    const rank = (t: Task) => (t.status === "done" ? 2 : prefer && t.repo === prefer ? 0 : 1);
    return tasks
      .filter((t) => t.id !== exclude && matchesQuery(t, query))
      .sort((a, b) => rank(a) - rank(b) || b.updated_at.localeCompare(a.updated_at))
      .slice(0, MAX);
  }, [tasks, exclude, prefer, query]);
  const current = found[Math.min(active, found.length - 1)];

  const pick = async (t: Task | undefined) => {
    if (!t || busy) return;
    setBusy(true);
    await onPick(t.id);
    setBusy(false);
  };

  return (
    <div className="palette-backdrop" onMouseDown={() => !busy && onCancel()}>
      <div
        className="palette picker"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onMouseDown={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          e.stopPropagation();
          if (e.key === "Escape") onCancel();
          else if (e.key === "ArrowDown") setActive((i) => Math.min(i + 1, found.length - 1));
          else if (e.key === "ArrowUp") setActive((i) => Math.max(i - 1, 0));
          else if (e.key === "Enter") void pick(current);
          else return;
          e.preventDefault();
        }}
      >
        <div className="picker-head">
          <h2>{title}</h2>
          {hint && <p>{hint}</p>}
        </div>
        <div className="palette-search">
          <input
            ref={input}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            placeholder="Search tasks by title, #id, Jira or repo"
            aria-label="Search tasks"
            aria-controls={listId}
            aria-activedescendant={current ? `${listId}-${current.id}` : undefined}
            role="combobox"
            aria-expanded="true"
          />
        </div>
        <ul className="palette-list" id={listId} role="listbox" aria-label="Tasks">
          {found.length === 0 && <li className="palette-empty">No task matches “{query}”.</li>}
          {found.map((t, i) => (
            <li
              key={t.id}
              id={`${listId}-${t.id}`}
              role="option"
              aria-selected={t === current}
              className={"palette-item" + (t === current ? " active" : "")}
              onMouseEnter={() => setActive(i)}
              onClick={() => void pick(t)}
            >
              <span className={`palette-icon st-${t.status}`} aria-hidden="true">
                {STATUS[t.status].icon}
              </span>
              <span className={"palette-label" + (t.status === "done" ? " is-done" : "")}>{t.title}</span>
              <span className="palette-meta">
                {t.repo && <span>{t.repo}</span>}
                <span className="mono">#{t.id}</span>
              </span>
            </li>
          ))}
        </ul>
        <div className="picker-foot">
          <span className="muted">↑↓ to move · ↩ to {actionLabel.toLowerCase()}</span>
          <button className="primary" disabled={!current || busy} onClick={() => void pick(current)}>
            {actionLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
