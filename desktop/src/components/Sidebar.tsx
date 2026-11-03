import type { ReactNode } from "react";
import { Mark } from "../Mark";
import { VIEWS, DONE } from "../model";

export type PageName = "list" | "new" | "edit" | "repos" | "settings" | "legend" | "unfiled" | "todos" | "usage";

interface Props {
  counts: { total: number; unread: number }[];
  /** Captured sessions waiting to be filed. */
  unfiled: number;
  view: number;
  page: PageName;
  onView: (i: number) => void;
  onPage: (p: PageName) => void;
  /** The notification bell, beside the name. */
  bell?: ReactNode;
}

export function Sidebar({ counts, unfiled, view, page, onView, onPage, bell }: Props) {
  return (
    <nav className="sidebar" aria-label="Views">
      <div className="drag sidebar-top" data-tauri-drag-region />
      <div className="brand">
        <Mark size={20} />
        <span>Tars</span>
        {bell && <span className="brand-bell">{bell}</span>}
      </div>
      <button
        className={"new-task-button" + (page === "new" ? " on" : "")}
        onClick={() => onPage("new")}
        title="New task (⌘N)"
      >
        <span aria-hidden="true">+</span>
        New task
        <kbd>⌘N</kbd>
      </button>
      <ul className="views">
        {VIEWS.map((v, i) => {
          const on = page === "list" && i === view;
          return (
            <li key={v.name}>
              <button
                className={"view" + (on ? " on" : "")}
                aria-current={on ? "page" : undefined}
                onClick={() => onView(i)}
                title={`${v.name} (${i + 1})`}
              >
                <span className="view-name">{v.name}</span>
                {counts[i]?.unread > 0 && i !== DONE && (
                  <span className="dot" aria-label={`${counts[i].unread} updated by an agent`}>
                    ●
                  </span>
                )}
                <span className="view-count" data-testid={`count-${i}`}>
                  {counts[i]?.total ?? 0}
                </span>
              </button>
            </li>
          );
        })}
        <li className="views-gap">
          <button
            className={"view" + (page === "unfiled" ? " on" : "")}
            aria-current={page === "unfiled" ? "page" : undefined}
            onClick={() => onPage("unfiled")}
            title="Sessions that changed code but didn't match a task"
          >
            <span className="view-name">Unfiled</span>
            {unfiled > 0 && (
              <span className="dot" aria-label={`${unfiled} sessions to file`}>
                ●
              </span>
            )}
            <span className="view-count" data-testid="count-unfiled">
              {unfiled}
            </span>
          </button>
        </li>
      </ul>
      <div className="sidebar-foot">
        <button
          className={"foot-link" + (page === "todos" ? " on" : "")}
          aria-current={page === "todos" ? "page" : undefined}
          onClick={() => onPage("todos")}
          title="Named checklists, separate from tasks"
        >
          <svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true">
            <path
              d="M2 3.5l1 1 1.8-2M2 8l1 1 1.8-2M6.8 3.6h5.2M6.8 8.1h5.2M6.8 11.6h5.2M2.6 11.6h.1"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.3"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          Todo lists
        </button>
        <button
          className={"foot-link" + (page === "usage" ? " on" : "")}
          aria-current={page === "usage" ? "page" : undefined}
          onClick={() => onPage("usage")}
          title="What your Claude sessions cost"
        >
          <svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true">
            <path d="M2 12h10M3.6 12V8.4M7 12V4.2M10.4 12V6.6" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
          </svg>
          Usage
        </button>
        <button
          className={"foot-link" + (page === "repos" ? " on" : "")}
          aria-current={page === "repos" ? "page" : undefined}
          onClick={() => onPage("repos")}
          title="Repos and preferences"
        >
          <svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true">
            <path
              d="M1.8 3.6c0-.6.4-1 1-1h2.6l1.2 1.3h4.6c.6 0 1 .4 1 1v5.5c0 .6-.4 1-1 1H2.8c-.6 0-1-.4-1-1V3.6Z"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.3"
              strokeLinejoin="round"
            />
          </svg>
          Repos
        </button>
        <button
          className={"foot-link" + (page === "legend" ? " on" : "")}
          aria-current={page === "legend" ? "page" : undefined}
          onClick={() => onPage("legend")}
          title="Legend (⌘/)"
        >
          <svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true">
            <circle cx="7" cy="7" r="5.6" fill="none" stroke="currentColor" strokeWidth="1.3" />
            <path d="M5.4 5.5a1.7 1.7 0 1 1 2.3 1.6c-.5.2-.7.6-.7 1.1M7 10.1v.1" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
          </svg>
          Legend
        </button>
        <button
          className={"foot-link" + (page === "settings" ? " on" : "")}
          aria-current={page === "settings" ? "page" : undefined}
          onClick={() => onPage("settings")}
          title="Settings (⌘,)"
        >
          <svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true">
            <circle cx="7" cy="7" r="2" fill="none" stroke="currentColor" strokeWidth="1.3" />
            <path
              d="M7 1.2v1.6M7 11.2v1.6M12.8 7h-1.6M2.8 7H1.2M11.1 2.9 10 4M4 10l-1.1 1.1M11.1 11.1 10 10M4 4 2.9 2.9"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.3"
              strokeLinecap="round"
            />
          </svg>
          Settings
        </button>
      </div>
    </nav>
  );
}
