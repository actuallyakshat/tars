import { useState } from "react";
import type { Capture } from "../api";
import { ago } from "../model";
import { Page } from "./Pages";

const shortPath = (p: string) => p.replace(/^\/Users\/[^/]+(?=\/|$)/, "~");
const FILES_SHOWN = 4;

/** What to call a session: its title, else the first line of its first prompt. */
export function captureTitle(c: Capture) {
  if (c.name) return c.name;
  const line = (c.prompt ?? "").split("\n")[0].trim();
  if (line) return line.length > 80 ? line.slice(0, 80) + "…" : line;
  return `Work in ${c.repo ?? "a repo"}`;
}

// Claude sessions that changed code but didn't match a task. Each one is
// filed into a task, made a new task, or dismissed.
export function UnfiledPage({
  sessions,
  captureOn,
  now,
  onFileInto,
  onNewTask,
  onDismiss,
  onResume,
  onSettings,
  onBack,
}: {
  sessions: Capture[];
  captureOn: boolean | null;
  now: Date;
  onFileInto: (c: Capture) => void;
  onNewTask: (c: Capture, title: string) => Promise<boolean>;
  onDismiss: (c: Capture) => void;
  onResume: (c: Capture) => void;
  onSettings: () => void;
  onBack: () => void;
}) {
  return (
    <Page title="Unfiled" onBack={onBack}>
      <p className="page-lede">
        Claude sessions that changed code but didn't match a task. File each into a task, make it a new one, or dismiss it.
      </p>
      {captureOn === false && (
        <p className="unfiled-off">
          Capture is off, so new sessions aren't recorded.{" "}
          <button className="link" onClick={onSettings}>
            Turn it on in Settings
          </button>
        </p>
      )}
      {sessions.length === 0 ? (
        <div className="unfiled-empty">
          <p className="unfiled-empty-title">Nothing to file</p>
          <p className="muted">
            Sessions linked to a task go straight to it. Ones that change code without a match wait here, and ones that
            only read or answer questions leave nothing behind.
          </p>
        </div>
      ) : (
        <ul className="unfiled-list" aria-label="Unfiled sessions">
          {sessions.map((c) => (
            <SessionCard
              key={c.session_id}
              c={c}
              now={now}
              onFileInto={() => onFileInto(c)}
              onNewTask={(title) => onNewTask(c, title)}
              onDismiss={() => onDismiss(c)}
              onResume={() => onResume(c)}
            />
          ))}
        </ul>
      )}
    </Page>
  );
}

function SessionCard({
  c,
  now,
  onFileInto,
  onNewTask,
  onDismiss,
  onResume,
}: {
  c: Capture;
  now: Date;
  onFileInto: () => void;
  onNewTask: (title: string) => Promise<boolean>;
  onDismiss: () => void;
  onResume: () => void;
}) {
  const title = captureTitle(c);
  const [naming, setNaming] = useState(false);
  const [draft, setDraft] = useState(title);
  const [busy, setBusy] = useState(false);
  const files = c.files ?? [];
  const prompt = (c.prompt ?? "").trim();
  const showPrompt = prompt && prompt.split("\n")[0].trim() !== title;

  const create = async () => {
    if (!draft.trim() || busy) return;
    setBusy(true);
    const ok = await onNewTask(draft.trim());
    setBusy(false);
    if (ok) setNaming(false);
  };

  return (
    <li className="unfiled-card" aria-label={title}>
      <div className="unfiled-where">
        {c.repo && <span className="unfiled-repo">{c.repo}</span>}
        {c.branch && <span className="mono">{c.branch}</span>}
        <span>{ago(c.ended_at || c.started_at, now)}</span>
      </div>
      <h2 className="unfiled-title">{title}</h2>
      {showPrompt && <p className="unfiled-prompt">{prompt}</p>}
      <p className="unfiled-work">
        <span>
          {c.edits} {c.edits === 1 ? "edit" : "edits"}
        </span>
        {c.commits > 0 && (
          <span>
            {c.commits} {c.commits === 1 ? "commit" : "commits"}
          </span>
        )}
        {files.length > 0 && (
          <span className="mono unfiled-files" title={files.join("\n")}>
            {files.slice(0, FILES_SHOWN).join(", ")}
            {files.length > FILES_SHOWN && ` +${files.length - FILES_SHOWN}`}
          </span>
        )}
      </p>
      {naming ? (
        <form
          className="unfiled-name"
          onSubmit={(e) => {
            e.preventDefault();
            void create();
          }}
        >
          <input
            autoFocus
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                e.stopPropagation();
                setNaming(false);
              }
            }}
            aria-label="New task title"
          />
          <button type="button" className="ghost" onClick={() => setNaming(false)}>
            Cancel
          </button>
          <button type="submit" className="primary" disabled={!draft.trim() || busy}>
            Create task
          </button>
        </form>
      ) : (
        <div className="unfiled-actions">
          <button className="primary" onClick={onFileInto}>
            File into a task…
          </button>
          <button className="ghost" onClick={() => setNaming(true)}>
            New task
          </button>
          <button className="ghost" onClick={onDismiss}>
            Dismiss
          </button>
          <button
            className="link unfiled-resume"
            onClick={onResume}
            title={c.worktree ? `claude --resume ${c.session_id} in ${shortPath(c.worktree)}` : undefined}
          >
            Resume in Terminal
          </button>
        </div>
      )}
    </li>
  );
}
