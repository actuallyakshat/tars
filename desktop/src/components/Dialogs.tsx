import { useEffect, useRef, useState } from "react";
import type { CliError, CliStatus, NewTask } from "../api";
import { Mark } from "../Mark";

function Sheet({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) {
  return (
    <div
      className="scrim"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          e.stopPropagation();
          onClose();
        }
      }}
    >
      <div className="sheet" role="dialog" aria-modal="true" aria-label={title}>
        <h2>{title}</h2>
        {children}
      </div>
    </div>
  );
}

export function AddTask({ onAdd, onClose }: { onAdd: (t: NewTask) => Promise<boolean>; onClose: () => void }) {
  const [t, setT] = useState<NewTask>({ title: "", priority: "normal" });
  const [busy, setBusy] = useState(false);
  const titleRef = useRef<HTMLInputElement>(null);
  useEffect(() => titleRef.current?.focus(), []);
  const set = (k: keyof NewTask) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) =>
    setT({ ...t, [k]: e.target.value });

  return (
    <Sheet title="New task" onClose={onClose}>
      <form
        className="form"
        onSubmit={async (e) => {
          e.preventDefault();
          if (!t.title.trim() || busy) return;
          setBusy(true);
          const ok = await onAdd(t);
          setBusy(false);
          if (ok) onClose();
        }}
      >
        <label className="wide">
          Title
          <input ref={titleRef} value={t.title} onChange={set("title")} placeholder="Fix CSV export timeout" />
        </label>
        <label>
          Priority
          <select value={t.priority} onChange={set("priority")}>
            <option value="urgent">Urgent</option>
            <option value="high">High</option>
            <option value="normal">Normal</option>
            <option value="low">Low</option>
          </select>
        </label>
        <label>
          Due
          <input value={t.due ?? ""} onChange={set("due")} placeholder="fri, +3d, 2026-11-03" />
        </label>
        <label>
          Jira
          <input value={t.jira ?? ""} onChange={set("jira")} placeholder="BILL-881" className="mono" />
        </label>
        <label>
          Repo
          <input value={t.repo ?? ""} onChange={set("repo")} placeholder="billing-svc" />
        </label>
        <label className="wide">
          Tags
          <input value={t.tags ?? ""} onChange={set("tags")} placeholder="perf, backend" />
        </label>
        <label className="wide">
          Description
          <textarea value={t.body ?? ""} onChange={set("body")} rows={3} />
        </label>
        <div className="form-actions wide">
          <button type="button" className="ghost" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="primary" disabled={!t.title.trim() || busy}>
            Add task
          </button>
        </div>
      </form>
    </Sheet>
  );
}

interface SettingsProps {
  jiraBase: string;
  cli: CliStatus | null;
  onSave: (jiraBase: string) => void;
  onClose: () => void;
}

export function Settings({ jiraBase, cli, onSave, onClose }: SettingsProps) {
  const [base, setBase] = useState(jiraBase);
  return (
    <Sheet title="Settings" onClose={onClose}>
      <form
        className="form"
        onSubmit={(e) => {
          e.preventDefault();
          onSave(base.trim());
          onClose();
        }}
      >
        <label className="wide">
          Jira URL
          <input
            autoFocus
            value={base}
            onChange={(e) => setBase(e.target.value)}
            placeholder="https://acme.atlassian.net"
            className="mono"
          />
          <span className="hint">Jira keys open as &lt;URL&gt;/browse/KEY. Leave empty to copy the key instead.</span>
        </label>
        {cli && (
          <p className="wide hint">
            Using <span className="mono">{cli.bin}</span> (version {cli.version})
          </p>
        )}
        <div className="form-actions wide">
          <button type="button" className="ghost" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="primary">
            Save
          </button>
        </div>
      </form>
    </Sheet>
  );
}

export function CliProblem({ error, onRetry }: { error: CliError; onRetry: () => void }) {
  const title = error.kind === "too_old" ? "Your tars CLI needs an update" : "tars needs its command-line tool";
  return (
    <main className="problem" role="alert">
      <div className="drag problem-top" data-tauri-drag-region />
      <Mark size={40} />
      <h1>{title}</h1>
      <p>{error.message}</p>
      <p className="hint">
        The app reads and changes tasks only through <span className="mono">tars</span>, and needs a build that has{" "}
        <span className="mono">tars report</span> (newer than 0.1.3). Set <span className="mono">TARS_BIN</span> to use
        a specific binary.
      </p>
      <button className="primary" onClick={onRetry}>
        Try again
      </button>
    </main>
  );
}
