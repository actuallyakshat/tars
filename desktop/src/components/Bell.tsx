import { useEffect, useRef, useState } from "react";
import type { Capture } from "../api";
import { ago } from "../model";
import { capturedAt } from "../fresh";
import { captureTitle } from "./UnfiledPage";

const SHOWN = 30;

// The bell in the sidebar: lists recently captured sessions, newest first.
// Its red dot means something was captured since it was last opened.
export function Bell({
  captured,
  lit,
  isNew,
  taskTitle,
  now,
  onOpen,
  onPick,
}: {
  captured: Capture[];
  lit: boolean;
  isNew: (c: Capture) => boolean;
  taskTitle: (id: number) => string | undefined;
  now: Date;
  onOpen: () => void;
  onPick: (c: Capture) => void;
}) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState({ top: 0, left: 0 });
  const button = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node;
      if (!panel.current?.contains(t) && !button.current?.contains(t)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    panel.current?.focus();
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const toggle = () => {
    if (open) return setOpen(false);
    const r = button.current?.getBoundingClientRect();
    if (r) setPos({ top: r.bottom + 6, left: Math.max(8, r.left - 8) });
    setOpen(true);
    onOpen();
  };

  const items = [...captured].sort((a, b) => capturedAt(b) - capturedAt(a)).slice(0, SHOWN);
  const newCount = captured.filter(isNew).length;

  return (
    <>
      <button
        ref={button}
        className={"bell" + (open ? " on" : "")}
        aria-label={lit ? "Captured sessions, new ones since you last looked" : "Captured sessions"}
        aria-expanded={open}
        aria-haspopup="dialog"
        title="Recently captured sessions"
        onClick={toggle}
      >
        <svg width="15" height="15" viewBox="0 0 16 16" aria-hidden="true">
          <path
            d="M4 6.6a4 4 0 0 1 8 0v2.6l1.2 2H2.8l1.2-2V6.6ZM6.4 13.2a1.7 1.7 0 0 0 3.2 0"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.3"
            strokeLinejoin="round"
            strokeLinecap="round"
          />
        </svg>
        {lit && <span className="bell-dot" data-testid="bell-dot" />}
      </button>
      {open && (
        <div
          ref={panel}
          className="bell-panel"
          role="dialog"
          aria-label="Recently captured"
          tabIndex={-1}
          style={pos}
          onKeyDown={(e) => {
            // Keys here are for the panel, not the task list behind it.
            e.stopPropagation();
            if (e.key === "Escape") {
              setOpen(false);
              button.current?.focus();
            }
          }}
        >
          <header className="bell-head">
            <span>Recently captured</span>
            <span className="bell-count">{newCount > 0 ? `${newCount} new` : "This week"}</span>
          </header>
          {items.length === 0 ? (
            <p className="bell-empty">Nothing captured this week. Claude sessions that change code show up here.</p>
          ) : (
            <ul className="bell-list">
              {items.map((c) => {
                const linked = c.state === "linked" && !!c.task_id;
                const itemIsNew = isNew(c);
                return (
                  <li key={c.session_id}>
                    <button
                      className={"bell-item" + (itemIsNew ? " is-new" : "")}
                      onClick={() => {
                        setOpen(false);
                        onPick(c);
                      }}
                    >
                      <span className="bell-row">
                        <span className="bell-title">{captureTitle(c)}</span>
                        {itemIsNew && <span className="new-badge">New</span>}
                        <span className="bell-time">{ago(c.ended_at || c.started_at, now)}</span>
                      </span>
                      <span className="bell-meta">
                        {linked ? (
                          <>
                            <span className="bell-ref">#{c.task_id}</span>
                            <span className="bell-task">{taskTitle(c.task_id!) ?? ""}</span>
                          </>
                        ) : (
                          <span className="bell-unfiled">Unfiled</span>
                        )}
                        {c.repo && <span className="bell-repo">{c.repo}</span>}
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}
    </>
  );
}
