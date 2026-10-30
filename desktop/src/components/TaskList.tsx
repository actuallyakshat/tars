import { useEffect, useRef } from "react";
import type { Task } from "../api";
import { type Group, NEEDS_YOU, OPEN, STATUS, VIEWS, ago, dueText, priorityMark } from "../model";

interface Props {
  groups: Group[];
  view: number;
  totalOpen: number;
  selectedId: number | null;
  now: Date;
  onSelect: (id: number) => void;
  onOpen: (id: number) => void;
}

function emptyMessage(view: number, totalOpen: number) {
  if (view === NEEDS_YOU) return "Nothing needs you. When an agent gets stuck, its question shows up here.";
  if (view === OPEN && totalOpen === 0) return "No open tasks. Press c to add one.";
  return "Nothing here.";
}

export function TaskList({ groups, view, totalOpen, selectedId, now, onSelect, onOpen }: Props) {
  const selRef = useRef<HTMLLIElement>(null);
  useEffect(() => {
    selRef.current?.scrollIntoView?.({ block: "nearest" });
  }, [selectedId]);

  return (
    <section className="list" aria-label={VIEWS[view].name}>
      <header className="list-head drag" data-tauri-drag-region>
        <h1>{VIEWS[view].name}</h1>
      </header>
      {groups.length === 0 ? (
        <p className="empty">{emptyMessage(view, totalOpen)}</p>
      ) : (
        <div className="list-scroll">
          {groups.map((g) => (
            <div key={g.label} className="group">
              <h2 className="group-head">
                {g.label} <span>· {g.tasks.length}</span>
              </h2>
              <ul role="listbox" aria-label={`${g.label} priority`}>
                {g.tasks.map((t) => (
                  <Row
                    key={t.id}
                    t={t}
                    now={now}
                    selected={t.id === selectedId}
                    ref={t.id === selectedId ? selRef : undefined}
                    onSelect={onSelect}
                    onOpen={onOpen}
                  />
                ))}
              </ul>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

interface RowProps {
  t: Task;
  now: Date;
  selected: boolean;
  ref?: React.Ref<HTMLLIElement>;
  onSelect: (id: number) => void;
  onOpen: (id: number) => void;
}

function Row({ t, now, selected, ref, onSelect, onOpen }: RowProps) {
  const due = t.status === "done" ? { text: ago(t.done_at, now), tone: "muted" } : dueText(t.due, now);
  const cls = ["row", `p${t.priority}`, `s-${t.status}`];
  if (selected) cls.push("sel");
  if (t.unread) cls.push("unread");
  return (
    <li
      ref={ref}
      role="option"
      aria-selected={selected}
      data-testid={`row-${t.id}`}
      className={cls.join(" ")}
      onClick={() => onSelect(t.id)}
      onDoubleClick={() => onOpen(t.id)}
    >
      <span className="c-mark">
        {t.needs_you ? (
          <span className="q" title="An agent is waiting on your answer">
            ?
          </span>
        ) : t.unread ? (
          <span className="dot" title="Updated by an agent since you last looked">
            ●
          </span>
        ) : null}
      </span>
      <span className="c-prio" aria-hidden="true">
        {priorityMark(t.priority)}
      </span>
      <span className={`c-status st-${t.status}`} title={STATUS[t.status].label}>
        {STATUS[t.status].icon}
      </span>
      <span className="c-id">#{t.id}</span>
      <span className="c-title">
        <span className="title-text">{t.title}</span>
        {t.status === "blocked" && t.waiting_on && <span className="waiting">waiting on {t.waiting_on}</span>}
      </span>
      <span className="c-jira">{t.jira}</span>
      <span className="c-repo">{t.repo}</span>
      <span className={`c-due tone-${due.tone}`}>{due.text}</span>
    </li>
  );
}
