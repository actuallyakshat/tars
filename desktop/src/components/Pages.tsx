import { useEffect, useRef, useState, type ReactNode } from "react";
import type { CliStatus, NewTask, Repo, Task, TaskEdit } from "../api";
import { SidebarToggle } from "./Panes";
import { PRIORITIES, STATUS, STATUSES, clampPriority, priorityMark } from "../model";

// Full pages that replace the list and detail panes: New task, Settings and
// Legend. Esc or the back button returns to the list.

function Page({ title, onBack, children, full }: { title: string; onBack: () => void; children: ReactNode; full?: boolean }) {
  return (
    <main className="page" aria-label={title}>
      <div className="drag page-top" data-tauri-drag-region>
        <SidebarToggle />
        <button className="back" onClick={onBack} title="Back (Esc)">
          <svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true">
            <path d="M8.5 3 4.5 7l4 4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          Back
        </button>
      </div>
      <div className="page-scroll">
        <div className={"page-body" + (full ? " full" : "")}>
          <h1 className="page-title">{title}</h1>
          {children}
        </div>
      </div>
    </main>
  );
}

const DUE_CHIPS = [
  ["Today", "today"],
  ["Tomorrow", "tomorrow"],
  ["Friday", "fri"],
  ["Next week", "+1w"],
] as const;

// The fields both New task and Edit task fill in, as typed.
type Draft = Required<Omit<NewTask, "priority">> & { priority: string };

const EMPTY: Draft = { title: "", body: "", priority: "normal", due: "", jira: "", repo: "", tags: "" };

const tagList = (s: string) => [...new Set(s.split(",").map((t) => t.trim()).filter(Boolean))];

function TaskForm({
  heading,
  initial,
  repos,
  submitLabel,
  onSubmit,
  onBack,
}: {
  heading: string;
  initial: Draft;
  repos: string[];
  submitLabel: string;
  onSubmit: (d: Draft) => Promise<boolean>;
  onBack: () => void;
}) {
  const [t, setT] = useState(initial);
  const [busy, setBusy] = useState(false);
  const titleRef = useRef<HTMLInputElement>(null);
  useEffect(() => titleRef.current?.focus(), []);
  const set = <K extends keyof Draft>(k: K, v: Draft[K]) => setT((x) => ({ ...x, [k]: v }));

  const submit = async () => {
    if (!t.title.trim() || busy) return;
    setBusy(true);
    const ok = await onSubmit({ ...t, title: t.title.trim() });
    setBusy(false);
    if (ok) onBack();
  };

  return (
    <Page title={heading} onBack={onBack}>
      <form
        className="new-task"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter" && e.metaKey) {
            e.preventDefault();
            void submit();
          }
        }}
      >
        <input
          ref={titleRef}
          className="title-input"
          aria-label="Title"
          value={t.title}
          onChange={(e) => set("title", e.target.value)}
          placeholder="What needs doing?"
        />
        <textarea
          className="body-input"
          aria-label="Description"
          value={t.body}
          onChange={(e) => set("body", e.target.value)}
          placeholder="Context, links, what done looks like. Optional."
          rows={4}
        />

        <div className="field-grid">
          <div className="field">
            <span className="field-label" id="prio-label">
              Priority
            </span>
            <div className="segmented" role="radiogroup" aria-labelledby="prio-label">
              {PRIORITIES.map((name, i) => {
                const value = name.toLowerCase();
                const on = t.priority === value;
                return (
                  <button
                    type="button"
                    key={name}
                    role="radio"
                    aria-checked={on}
                    className={(on ? "on " : "") + `p${i}`}
                    onClick={() => set("priority", value)}
                  >
                    {priorityMark(i) && <span aria-hidden="true">{priorityMark(i)}</span>}
                    {name}
                  </button>
                );
              })}
            </div>
          </div>

          <label className="field">
            <span className="field-label">Due</span>
            <input value={t.due} onChange={(e) => set("due", e.target.value)} placeholder="fri, +3d, 2026-11-03" />
            <span className="chips">
              {DUE_CHIPS.map(([label, value]) => (
                <button
                  type="button"
                  key={value}
                  className={"chip" + (t.due === value ? " on" : "")}
                  onClick={() => set("due", t.due === value ? "" : value)}
                >
                  {label}
                </button>
              ))}
            </span>
          </label>

          <label className="field">
            <span className="field-label">Jira</span>
            <input className="mono" value={t.jira} onChange={(e) => set("jira", e.target.value)} placeholder="BILL-881" />
          </label>

          <label className="field">
            <span className="field-label">Repo</span>
            <input list="repo-options" value={t.repo} onChange={(e) => set("repo", e.target.value)} placeholder="billing-svc" />
            <datalist id="repo-options">
              {repos.map((r) => (
                <option key={r} value={r} />
              ))}
            </datalist>
          </label>

          <label className="field span-2">
            <span className="field-label">Tags</span>
            <input value={t.tags} onChange={(e) => set("tags", e.target.value)} placeholder="perf, backend" />
          </label>
        </div>

        <div className="page-actions">
          <button type="button" className="ghost" onClick={onBack}>
            Cancel
          </button>
          <button type="submit" className="primary" disabled={!t.title.trim() || busy}>
            {submitLabel} <kbd aria-hidden="true">⌘↩</kbd>
          </button>
        </div>
      </form>
    </Page>
  );
}

export function NewTaskPage({
  repos,
  onAdd,
  onBack,
}: {
  repos: string[];
  onAdd: (t: NewTask) => Promise<boolean>;
  onBack: () => void;
}) {
  // Only the fields filled in.
  const submit = (d: Draft) =>
    onAdd(Object.fromEntries(Object.entries(d).filter(([, v]) => v.trim())) as unknown as NewTask);
  return <TaskForm heading="New task" initial={EMPTY} repos={repos} submitLabel="Create task" onSubmit={submit} onBack={onBack} />;
}

/** Only what changed, in the shape `tars edit` takes. */
export function taskChanges(task: Task, d: Draft): TaskEdit {
  const before = draftOf(task);
  const e: TaskEdit = {};
  for (const k of ["title", "body", "jira", "repo", "priority"] as const) {
    if (d[k].trim() !== before[k]) e[k] = d[k].trim();
  }
  if (d.due.trim() !== before.due) e.due = d.due.trim() || "none";
  const was = tagList(before.tags);
  const now = tagList(d.tags);
  const tag = now.filter((x) => !was.includes(x));
  const untag = was.filter((x) => !now.includes(x));
  if (tag.length) e.tag = tag;
  if (untag.length) e.untag = untag;
  return e;
}

function draftOf(t: Task): Draft {
  return {
    title: t.title,
    body: t.body ?? "",
    priority: PRIORITIES[clampPriority(t.priority)].toLowerCase(),
    due: t.due ?? "",
    jira: t.jira ?? "",
    repo: t.repo ?? "",
    tags: (t.tags ?? []).join(", "),
  };
}

export function EditTaskPage({
  task,
  repos,
  onSave,
  onBack,
}: {
  task: Task;
  repos: string[];
  onSave: (e: TaskEdit) => Promise<boolean>;
  onBack: () => void;
}) {
  return (
    <TaskForm
      heading={`Edit #${task.id}`}
      initial={draftOf(task)}
      repos={repos}
      submitLabel="Save changes"
      onSubmit={(d) => onSave(taskChanges(task, d))}
      onBack={onBack}
    />
  );
}

type SettingsSection = "general" | "repos" | "integrations" | "shortcuts";

export interface RepoActions {
  repos: Repo[];
  onSetRepo: (name: string, path: string) => Promise<boolean>;
  onRemoveRepo: (name: string) => Promise<boolean>;
  onChooseFolder: () => Promise<string | null>;
}

export function SettingsPage({
  initialSection = "general",
  jiraBase,
  cli,
  onSaveJira,
  onBack,
  ...repoActions
}: {
  jiraBase: string;
  cli: CliStatus | null;
  onSaveJira: (base: string) => void;
  onBack: () => void;
  initialSection?: SettingsSection;
} & RepoActions) {
  const [section, setSection] = useState<SettingsSection>(initialSection);
  const [base, setBase] = useState(jiraBase);
  const [saved, setSaved] = useState(false);
  const dirty = base.trim() !== jiraBase;

  return (
    <Page title="Settings" onBack={onBack} full>
      <div className="settings">
        <nav className="settings-nav" aria-label="Settings sections">
          {(
            [
              ["general", "General"],
              ["repos", "Repos"],
              ["integrations", "Integrations"],
              ["shortcuts", "Keyboard shortcuts"],
            ] as const
          ).map(([id, label]) => (
            <button key={id} className={section === id ? "on" : ""} aria-current={section === id ? "page" : undefined} onClick={() => setSection(id)}>
              {label}
            </button>
          ))}
        </nav>

        <div className="settings-content">
          {section === "general" && (
            <section aria-label="General">
              <h2>tars command-line tool</h2>
              <p className="muted">The app reads and changes tasks only through this binary.</p>
              <dl className="settings-rows">
                <dt>Binary</dt>
                <dd className="mono">{cli?.bin ?? "Not found"}</dd>
                <dt>Version</dt>
                <dd className="mono">{cli?.version ?? "—"}</dd>
              </dl>
              <p className="hint">
                To use a different build, start the app with <span className="mono">TARS_BIN=/path/to/tars</span>.
              </p>
            </section>
          )}

          {section === "repos" && <RepoSettings {...repoActions} />}

          {section === "integrations" && (
            <section aria-label="Integrations">
              <h2>Jira</h2>
              <p className="muted">With your Jira URL set, clicking a key opens the issue. Without it, the key is copied.</p>
              <form
                className="settings-form"
                onSubmit={(e) => {
                  e.preventDefault();
                  onSaveJira(base.trim());
                  setSaved(true);
                }}
              >
                <label className="field">
                  <span className="field-label">Jira URL</span>
                  <input
                    className="mono"
                    value={base}
                    onChange={(e) => {
                      setBase(e.target.value);
                      setSaved(false);
                    }}
                    placeholder="https://acme.atlassian.net"
                  />
                </label>
                <div className="settings-save">
                  <button type="submit" className="primary" disabled={!dirty}>
                    Save
                  </button>
                  {saved && !dirty && <span className="saved">Saved</span>}
                </div>
              </form>
            </section>
          )}

          {section === "shortcuts" && (
            <section aria-label="Keyboard shortcuts">
              <h2>Keyboard shortcuts</h2>
              <ShortcutTable />
            </section>
          )}
        </div>
      </div>
    </Page>
  );
}

const basename = (p: string) => p.replace(/\/+$/, "").split("/").pop() ?? "";

// Repo profiles: a name tasks use, and the folder it lives in, so agents
// started inside that folder pick up the right profile.
function RepoSettings({ repos, onSetRepo, onRemoveRepo, onChooseFolder }: RepoActions) {
  const [name, setName] = useState("");
  const [path, setPath] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState<string | null>(null);

  const add = async () => {
    if (!name.trim() || !path.trim() || busy) return;
    setBusy(true);
    const ok = await onSetRepo(name.trim(), path.trim());
    setBusy(false);
    if (ok) {
      setName("");
      setPath("");
    }
  };

  const browse = async () => {
    const dir = await onChooseFolder();
    if (!dir) return;
    setPath(dir);
    if (!name.trim()) setName(basename(dir));
  };

  const repoint = async (repo: string) => {
    const dir = await onChooseFolder();
    if (dir) await onSetRepo(repo, dir);
  };

  return (
    <section aria-label="Repos">
      <h2>Repos</h2>
      <p className="muted">
        Tell tars where each repo lives. Agents working inside that folder get its profile, and new tasks can pick it as
        their repo.
      </p>
      {repos.length === 0 ? (
        <p className="hint">No repos yet. Add the first one below.</p>
      ) : (
        <ul className="repo-list" aria-label="Your repos">
          {repos.map((r) => (
            <li key={r.name}>
              <div className="repo-text">
                <span className="repo-name">{r.name}</span>
                <span className={"repo-path mono" + (r.path ? "" : " muted")}>{r.path || "No folder set"}</span>
              </div>
              {confirming === r.name ? (
                <span className="repo-actions">
                  <span className="muted">Delete the profile and its notes?</span>
                  <button className="ghost" onClick={() => setConfirming(null)}>
                    Keep
                  </button>
                  <button
                    className="danger"
                    onClick={async () => {
                      setConfirming(null);
                      await onRemoveRepo(r.name);
                    }}
                  >
                    Delete {r.name}
                  </button>
                </span>
              ) : (
                <span className="repo-actions">
                  <button className="ghost" onClick={() => void repoint(r.name)}>
                    {r.path ? "Change folder…" : "Set folder…"}
                  </button>
                  <button className="ghost" onClick={() => setConfirming(r.name)} aria-label={`Remove ${r.name}`}>
                    Remove
                  </button>
                </span>
              )}
            </li>
          ))}
        </ul>
      )}

      <form
        className="repo-add"
        onSubmit={(e) => {
          e.preventDefault();
          void add();
        }}
      >
        <h3>Add a repo</h3>
        <div className="repo-add-row">
          <label className="field">
            <span className="field-label">Folder</span>
            <span className="path-field">
              <input className="mono" value={path} onChange={(e) => setPath(e.target.value)} placeholder="~/code/billing-svc" />
              <button type="button" className="ghost" onClick={() => void browse()}>
                Choose…
              </button>
            </span>
          </label>
          <label className="field">
            <span className="field-label">Name</span>
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="billing-svc" />
          </label>
        </div>
        <div className="settings-save">
          <button type="submit" className="primary" disabled={!name.trim() || !path.trim() || busy}>
            Add repo
          </button>
        </div>
      </form>
    </section>
  );
}

const SHORTCUTS: [string, string][] = [
  ["j  k  ↑  ↓", "Move through the list"],
  ["Enter", "Open the selected task"],
  ["Esc", "Back to the list, close the preview, or leave a page"],
  ["1 – 7", "Switch view"],
  ["x", "Mark done, or reopen"],
  ["s", "Change status"],
  ["p", "Change priority"],
  ["u", "Undo the last status or priority change"],
  ["/", "Filter the list"],
  ["g", "Change grouping"],
  ["Tab  ⇧Tab", "Next or previous view"],
  ["a", "Answer the agent's question"],
  ["n", "Add a note"],
  ["l", "Show or hide activity"],
  ["o", "Copy the resume command"],
  ["e", "Edit the selected task"],
  ["⌘⌫", "Delete the selected task (asks first)"],
  ["⌘K", "Search tasks and commands"],
  ["⌘N", "New task"],
  ["⌘B", "Show or hide the sidebar"],
  ["⌘⇧B", "Show or hide the preview"],
  ["f", "Open the selected task full screen, or go back"],
  ["⌘R", "Refresh"],
  ["⌘+  ⌘−  ⌘0", "Zoom in, zoom out, actual size"],
  ["?", "Legend"],
];

function ShortcutTable() {
  return (
    <dl className="shortcuts">
      {SHORTCUTS.map(([keys, what]) => (
        <div key={keys}>
          <dt>
            <kbd>{keys}</kbd>
          </dt>
          <dd>{what}</dd>
        </div>
      ))}
    </dl>
  );
}

const STATUS_HELP: Record<string, string> = {
  inbox: "Captured, not yet looked at closely.",
  ready: "Clear enough for an agent to work on without asking.",
  "in-progress": "Someone, you or an agent, is working on it.",
  "in-review": "Work is done and waiting for you to review it.",
  blocked: "Waiting on someone, or on your answer.",
  done: "Finished.",
};

const PRIORITY_HELP = ["Drop everything.", "Do it soon.", "The default. No mark, to keep the list quiet.", "Whenever there's time."];

export function LegendPage({ onBack }: { onBack: () => void }) {
  return (
    <Page title="Legend" onBack={onBack} full>
      <p className="lede">What every mark in the task list means.</p>

      <section className="legend" aria-label="Markers">
        <h2>Markers</h2>
        <dl>
          <div>
            <dt>
              <span className="mark-sample ask">?</span>
            </dt>
            <dd>
              <strong>Needs you.</strong> An agent left a question it can't answer on its own. Answer it with{" "}
              <kbd>a</kbd>; the next agent reads your answer.
            </dd>
          </div>
          <div>
            <dt>
              <span className="mark-sample unread">●</span>
            </dt>
            <dd>
              <strong>Agent update.</strong> An agent changed the task since you last looked. It clears when you open
              the task.
            </dd>
          </div>
          <div>
            <dt>
              <span className="mark-sample bold">Aa</span>
            </dt>
            <dd>
              <strong>Bold title.</strong> Same as the ●: there's something new to read.
            </dd>
          </div>
        </dl>
      </section>

      <section className="legend" aria-label="Status">
        <h2>Status</h2>
        <dl>
          {STATUSES.map((s) => (
            <div key={s}>
              <dt>
                <span className={`mark-sample st-${s}`}>{STATUS[s].icon}</span>
              </dt>
              <dd>
                <strong>{STATUS[s].label}.</strong> {STATUS_HELP[s]}
              </dd>
            </div>
          ))}
        </dl>
      </section>

      <section className="legend" aria-label="Priority">
        <h2>Priority</h2>
        <dl>
          {PRIORITIES.map((name, i) => (
            <div key={name}>
              <dt>
                <span className={`mark-sample p${i}`}>{priorityMark(i) || "—"}</span>
              </dt>
              <dd>
                <strong>{name}.</strong> {PRIORITY_HELP[i]}
              </dd>
            </div>
          ))}
        </dl>
      </section>

      <section className="legend" aria-label="Due dates">
        <h2>Due dates</h2>
        <dl>
          <div>
            <dt>
              <span className="mark-sample due-late">2d late</span>
            </dt>
            <dd>Past its due date.</dd>
          </div>
          <div>
            <dt>
              <span className="mark-sample due-today">today</span>
            </dt>
            <dd>Due today or tomorrow.</dd>
          </div>
          <div>
            <dt>
              <span className="mark-sample muted">Thu</span>
            </dt>
            <dd>Due later. A weekday means this week; further out shows the date.</dd>
          </div>
        </dl>
      </section>

      <section className="legend" aria-label="Keyboard shortcuts">
        <h2>Keyboard shortcuts</h2>
        <ShortcutTable />
      </section>
    </Page>
  );
}
