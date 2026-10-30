import { Mark } from "../Mark";
import { VIEWS, DONE } from "../model";

interface Props {
  counts: { total: number; unread: number }[];
  view: number;
  onView: (i: number) => void;
  onAdd: () => void;
  onSettings: () => void;
}

export function Sidebar({ counts, view, onView, onAdd, onSettings }: Props) {
  return (
    <nav className="sidebar" aria-label="Views">
      <div className="drag sidebar-top" data-tauri-drag-region />
      <div className="brand">
        <Mark size={20} />
        <span>tars</span>
      </div>
      <ul className="views">
        {VIEWS.map((v, i) => (
          <li key={v.name}>
            <button
              className={"view" + (i === view ? " on" : "")}
              aria-current={i === view ? "page" : undefined}
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
        ))}
      </ul>
      <div className="sidebar-foot">
        <button className="ghost" onClick={onAdd} title="New task (c)">
          + New task
        </button>
        <button className="ghost" onClick={onSettings} title="Settings">
          Settings
        </button>
      </div>
    </nav>
  );
}
