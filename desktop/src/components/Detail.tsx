import type { Report, Status, Task, TaskSession } from "../api";
import { ago, clampPriority, dueText, openQuestion } from "../model";
import { Composer } from "./Composer";
import { PriorityPicker, StatusPicker } from "./StatusPicker";

export interface DetailActions {
  onClose: () => void;
  onEdit: () => void;
  onStatus: (status: Status) => void;
  onPriority: (priority: number) => void;
  onToggleDone: () => void;
  onAnswer: (text: string) => Promise<boolean>;
  onNote: (text: string) => Promise<boolean>;
  onJira: (key: string) => void;
  onCopy: (text: string, what: string) => void;
  onReveal: (path: string) => void;
  onTerminal: (s: TaskSession) => void;
  setAnswering: (v: boolean) => void;
  setNoting: (v: boolean) => void;
  setLogOpen: (v: boolean) => void;
}

interface Props extends DetailActions {
  task: Task | null;
  now: Date;
  focused: boolean;
  answering: boolean;
  noting: boolean;
  logOpen: boolean;
}

export function Detail(p: Props) {
  const { task: t, now } = p;
  if (!t) {
    return (
      <section className="detail empty-detail" aria-label="Task details">
        <div className="drag detail-top" data-tauri-drag-region />
        <p className="empty">Select a task to see what the agents did.</p>
      </section>
    );
  }
  const prio = clampPriority(t.priority);
  return (
    <section className={"detail" + (p.focused ? " focused" : "")} aria-label="Task details" tabIndex={-1}>
      <div className="drag detail-top" data-tauri-drag-region />
      <div className="detail-scroll">
        <header className="detail-head">
          <span className="mono muted">#{t.id}</span>
          <StatusPicker value={t.status} onChange={p.onStatus} />
          <PriorityPicker value={prio} onChange={p.onPriority} />
          <span className="spacer" />
          <button className="ghost" onClick={p.onEdit} title="Edit (e)">
            Edit
          </button>
          <button className="ghost" onClick={p.onToggleDone} title="x">
            {t.status === "done" ? "Reopen" : "Mark done"}
          </button>
          <button className="icon-button" onClick={p.onClose} aria-label="Close preview" title="Close preview (⌘⇧B)">
            <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">
              <path d="M3 3l6 6M9 3l-6 6" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </header>

        <h2 className="detail-title">{t.title}</h2>

        {t.report && (
          <ReportCard
            r={t.report}
            now={now}
            open={!!openQuestion(t)}
            answering={p.answering}
            onAnswer={p.onAnswer}
            setAnswering={p.setAnswering}
          />
        )}

        <Fields {...p} t={t} now={now} />

        {t.body?.trim() && (
          <Section title="Description">
            <p className="prose">{t.body.trim()}</p>
          </Section>
        )}
        {t.acceptance?.trim() && (
          <Section title="Acceptance criteria">
            <p className="prose">{t.acceptance.trim()}</p>
          </Section>
        )}

        {!!t.sessions?.length && (
          <Section title="Sessions">
            <ul className="sessions">
              {t.sessions.map((s) => (
                <li key={s.id} className="session">
                  <div className="session-main">
                    <span className="session-name">{s.name || "Unnamed session"}</span>
                    <span className="muted"> · {ago(s.last_at, now)}</span>
                    <div className="mono muted small">
                      {s.launch_dir && <span title="Started in">{s.launch_dir} · </span>}
                      {s.id}
                    </div>
                  </div>
                  <div className="session-actions">
                    <button className="ghost" onClick={() => p.onCopy(`claude --resume ${s.id}`, "resume command")}>
                      Copy resume command
                    </button>
                    <button className="ghost" onClick={() => p.onTerminal(s)}>
                      Open in Terminal
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          </Section>
        )}

        <Section title="Notes">
          {p.noting ? (
            <Composer
              label={`Note on #${t.id}`}
              placeholder="Add a note to the log…"
              submitLabel="Add note"
              onSubmit={p.onNote}
              onCancel={() => p.setNoting(false)}
            />
          ) : (
            <button className="ghost" onClick={() => p.setNoting(true)} title="n">
              Add a note
            </button>
          )}
        </Section>

        <Activity t={t} now={now} open={p.logOpen} setOpen={p.setLogOpen} />
      </div>
    </section>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="section">
      <h3 className="section-head">{title}</h3>
      {children}
    </section>
  );
}

interface ReportProps {
  r: Report;
  now: Date;
  open: boolean;
  answering: boolean;
  onAnswer: (text: string) => Promise<boolean>;
  setAnswering: (v: boolean) => void;
}

function ReportCard({ r, now, open, answering, onAnswer, setAnswering }: ReportProps) {
  return (
    <section className={"report" + (open ? " asking" : "")} aria-label="Report">
      <div className="report-head">
        <span>Report</span>
        <span className="muted">{ago(r.at, now)}</span>
      </div>
      <dl className="report-lines">
        <dt>done</dt>
        <dd>{r.done}</dd>
        {r.verified && (
          <>
            <dt>verified</dt>
            <dd>{r.verified}</dd>
          </>
        )}
        {r.needs_you && (
          <>
            <dt className="needs">needs you</dt>
            <dd>
              {r.answered_at ? (
                <div className="qa">
                  <div>
                    <span className="muted">Asked: </span>
                    {r.needs_you}
                  </div>
                  <div>
                    <span className="muted">You answered: </span>
                    {r.answer}
                    <span className="muted"> · {ago(r.answered_at, now)}</span>
                  </div>
                </div>
              ) : (
                <span className="question">
                  <span className="q" aria-hidden="true">
                    ?
                  </span>{" "}
                  {r.needs_you}
                </span>
              )}
            </dd>
          </>
        )}
        {r.next && (
          <>
            <dt>next</dt>
            <dd>{r.next}</dd>
          </>
        )}
      </dl>
      {open &&
        (answering ? (
          <Composer
            label="Your answer"
            placeholder="Your answer. The next agent on this task will see it."
            submitLabel="Send answer"
            onSubmit={onAnswer}
            onCancel={() => setAnswering(false)}
          />
        ) : (
          <button className="primary" onClick={() => setAnswering(true)} title="a">
            Answer
          </button>
        ))}
    </section>
  );
}

function Fields({ t, now, onJira, onCopy, onReveal }: { t: Task; now: Date } & DetailActions) {
  const rows: [string, React.ReactNode][] = [];
  if (t.jira)
    rows.push([
      "jira",
      <button className="link mono" onClick={() => onJira(t.jira!)}>
        {t.jira}
      </button>,
    ]);
  if (t.repo) rows.push(["repo", t.repo]);
  if (t.due) {
    const d = dueText(t.due, now);
    rows.push([
      "due",
      <>
        <span className="mono">{t.due}</span> <span className={`tone-${d.tone}`}>{d.text}</span>
      </>,
    ]);
  }
  if (t.tags?.length) rows.push(["tags", <span className="tags">{t.tags.map((x) => `#${x}`).join("  ")}</span>]);
  if (t.waiting_on) rows.push(["waiting on", t.waiting_on]);
  if (t.follow_up) rows.push(["follow up", <span className="mono">{t.follow_up}</span>]);
  if (t.branch)
    rows.push([
      "branch",
      <button className="link mono" title="Copy branch name" onClick={() => onCopy(t.branch!, "branch name")}>
        {t.branch}
      </button>,
    ]);
  if (t.worktree)
    rows.push([
      "worktree",
      <button className="link mono" title="Reveal in Finder" onClick={() => onReveal(t.worktree!)}>
        {t.worktree}
      </button>,
    ]);
  return (
    <dl className="fields">
      {rows.map(([k, v]) => (
        <div key={k} className="field">
          <dt>{k}</dt>
          <dd>{v}</dd>
        </div>
      ))}
      <div className="field">
        <dt>created</dt>
        <dd className="muted">
          {ago(t.created_at, now)} · updated {ago(t.updated_at, now)}
        </dd>
      </div>
    </dl>
  );
}

function Activity({ t, now, open, setOpen }: { t: Task; now: Date; open: boolean; setOpen: (v: boolean) => void }) {
  const log = t.log ?? [];
  if (log.length === 0) return <p className="muted section">No activity yet</p>;
  const last = log[log.length - 1];
  return (
    <section className="section activity">
      <button className="section-head toggle" aria-expanded={open} onClick={() => setOpen(!open)} title="l">
        {open ? "▾" : "▸"} Activity
        {!open && (
          <span className="muted summary">
            {" "}
            · {log.length} {log.length === 1 ? "entry" : "entries"} · last: {last.author} · {last.kind} ·{" "}
            {ago(last.at, now)}
          </span>
        )}
      </button>
      {open && (
        <ol className="log">
          {[...log].reverse().map((e) => (
            <li key={e.id}>
              <div className="log-meta">
                <span className={e.author === "agent" ? "agent" : "muted"}>{e.author}</span>
                <span className="muted">
                  {" "}
                  · {e.kind} · {ago(e.at, now)}
                </span>
              </div>
              <p className="prose">{e.text.trim()}</p>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
