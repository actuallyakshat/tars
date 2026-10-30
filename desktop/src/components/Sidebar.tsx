import { Mark } from "../Mark";
import { VIEWS, DONE } from "../model";

export type PageName = "list" | "new" | "edit" | "repos" | "settings" | "legend";

interface Props {
  counts: { total: number; unread: number }[];
  view: number;
  page: PageName;
  onView: (i: number) => void;
  onPage: (p: PageName) => void;
}

export function Sidebar({ counts, view, page, onView, onPage }: Props) {
  return (
    <nav className="sidebar" aria-label="Views">
      <div className="drag sidebar-top" data-tauri-drag-region />
      <div className="brand">
        <Mark size={20} />
        <span>tars</span>
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
      </ul>
      <div className="sidebar-foot">
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
          title="Legend (?)"
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
          title="Settings"
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
