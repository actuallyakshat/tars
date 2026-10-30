import { useEffect, useRef, useState } from "react";
import type { Task } from "../api";
import { type Group, type GroupBy, type SortBy, GROUPINGS, NEEDS_YOU, OPEN, SORTS, STATUS, VIEWS, ago, dueText, prLabel, priorityMark } from "../model";
import { PreviewToggle, SidebarToggle } from "./Panes";

interface Props {
  groups: Group[];
  view: number;
  totalOpen: number;
  selectedId: number | null;
  now: Date;
  query: string;
  group: GroupBy;
  sort: SortBy;
  onQuery: (q: string) => void;
  onGroup: (g: GroupBy) => void;
  onSort: (s: SortBy) => void;
  onSelect: (id: number) => void;
  onOpen: (id: number) => void;
  onMenu: (id: number, x: number, y: number) => void;
  onOpenUrl: (url: string) => void;
}

function emptyMessage(view: number, totalOpen: number, query: string) {
  if (query.trim()) return `No tasks here match “${query.trim()}”. Esc clears the search.`;
  if (view === NEEDS_YOU) return "Nothing needs you. When an agent gets stuck, its question shows up here.";
  if (view === OPEN && totalOpen === 0) return "No open tasks. Press c to add one.";
  return "Nothing here.";
}

export function TaskList({ groups, view, totalOpen, selectedId, now, query, group, sort, onQuery, onGroup, onSort, onSelect, onOpen, onMenu, onOpenUrl }: Props) {
  const selRef = useRef<HTMLLIElement>(null);
  useEffect(() => {
    selRef.current?.scrollIntoView?.({ block: "nearest" });
  }, [selectedId]);

  return (
    <section className="list" aria-label={VIEWS[view].name}>
      <header className="list-head drag" data-tauri-drag-region>
        <SidebarToggle />
        <h1>{VIEWS[view].name}</h1>
        <span className="spacer" data-tauri-drag-region />
        <SearchBox query={query} onQuery={onQuery} />
        <DisplayMenu group={group} sort={sort} onGroup={onGroup} onSort={onSort} />
        <PreviewToggle />
      </header>
      {groups.length === 0 ? (
        <p className="empty">{emptyMessage(view, totalOpen, query)}</p>
      ) : (
        <div className="list-scroll">
          {groups.map((g) => (
            <div key={g.label} className="group">
              {g.label && (
                <h2 className="group-head">
                  {g.label} <span>· {g.tasks.length}</span>
                </h2>
              )}
              <ul role="listbox" aria-label={g.label || "Tasks"}>
                {g.tasks.map((t) => (
                  <Row
                    key={t.id}
                    t={t}
                    now={now}
                    selected={t.id === selectedId}
                    ref={t.id === selectedId ? selRef : undefined}
                    onSelect={onSelect}
                    onOpen={onOpen}
                    onMenu={onMenu}
                    onOpenUrl={onOpenUrl}
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
  onMenu: (id: number, x: number, y: number) => void;
  onOpenUrl: (url: string) => void;
}

function Row({ t, now, selected, ref, onSelect, onOpen, onMenu, onOpenUrl }: RowProps) {
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
      onContextMenu={(e) => {
        e.preventDefault();
        onMenu(t.id, e.clientX, e.clientY);
      }}
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
        {t.pr && (
          <button
            className="pr-chip"
            title={`Open ${t.pr}`}
            onClick={(e) => {
              e.stopPropagation();
              onOpenUrl(t.pr!);
            }}
          >
            <PrIcon />
            {prLabel(t.pr)}
          </button>
        )}
      </span>
      <span className="c-jira">{t.jira}</span>
      <span className="c-repo">{t.repo}</span>
      <span className={`c-due tone-${due.tone}`}>{due.text}</span>
    </li>
  );
}

// Filters the list as you type. / focuses it; Esc clears it.
function SearchBox({ query, onQuery }: { query: string; onQuery: (q: string) => void }) {
  return (
    <label className={"list-search" + (query ? " has-query" : "")}>
      <svg width="13" height="13" viewBox="0 0 15 15" aria-hidden="true">
        <circle cx="6.5" cy="6.5" r="4.6" fill="none" stroke="currentColor" strokeWidth="1.5" />
        <path d="m10 10 3.4 3.4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
      <input
        id="list-search"
        aria-label="Filter tasks"
        value={query}
        placeholder="Filter"
        spellCheck={false}
        onChange={(e) => onQuery(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            e.stopPropagation();
            onQuery("");
            e.currentTarget.blur();
          } else if (e.key === "Enter" || e.key === "ArrowDown") {
            e.currentTarget.blur();
          }
        }}
      />
      {!query && <kbd>/</kbd>}
    </label>
  );
}

// Group and sort choices for the list. g cycles the grouping from the keyboard.
function DisplayMenu({
  group,
  sort,
  onGroup,
  onSort,
}: {
  group: GroupBy;
  sort: SortBy;
  onGroup: (g: GroupBy) => void;
  onSort: (s: SortBy) => void;
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", away);
    return () => document.removeEventListener("mousedown", away);
  }, [open]);

  return (
    <div className="display-menu" ref={root}>
      <button
        className={"icon-button" + (open ? " on" : "")}
        aria-label="Display options"
        aria-expanded={open}
        title="Group and sort (g cycles grouping)"
        onClick={() => setOpen((o) => !o)}
      >
        <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
          <path d="M2.5 4.5h11M4.5 8h7M6.5 11.5h3" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
        </svg>
      </button>
      {open && (
        <div
          className="display-pop"
          role="dialog"
          aria-label="Display options"
          onKeyDown={(e) => {
            e.stopPropagation();
            if (e.key === "Escape") setOpen(false);
          }}
        >
          <fieldset>
            <legend>Group by</legend>
            {GROUPINGS.map((g) => (
              <label key={g.id} className={"display-option" + (g.id === group ? " on" : "")}>
                <input type="radio" name="group" checked={g.id === group} onChange={() => onGroup(g.id)} />
                {g.label}
              </label>
            ))}
          </fieldset>
          <fieldset>
            <legend>Sort by</legend>
            {SORTS.map((o) => (
              <label key={o.id} className={"display-option" + (o.id === sort ? " on" : "")}>
                <input type="radio" name="sort" checked={o.id === sort} onChange={() => onSort(o.id)} />
                {o.label}
              </label>
            ))}
          </fieldset>
        </div>
      )}
    </div>
  );
}

// A pull request: two branches joining.
export function PrIcon() {
  return (
    <svg width="11" height="11" viewBox="0 0 12 12" aria-hidden="true">
      <circle cx="3" cy="2.5" r="1.4" fill="none" stroke="currentColor" strokeWidth="1.2" />
      <circle cx="3" cy="9.5" r="1.4" fill="none" stroke="currentColor" strokeWidth="1.2" />
      <circle cx="9" cy="9.5" r="1.4" fill="none" stroke="currentColor" strokeWidth="1.2" />
      <path d="M3 4v4M9 8V5.5A2 2 0 0 0 7 3.5H5.5" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
    </svg>
  );
}
