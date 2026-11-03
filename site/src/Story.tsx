import { useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Reveal } from "./Reveal";
import { TuiDemo } from "./TuiDemo";

// The sections that explain Tars, in the order a new reader needs them:
// how capture decides, what a task collects, what waits for you, and where you read it.

const signals: [string, string, string][] = [
  ["A branch named for a task", "tars/15-webhook-retry", "Links to #15"],
  ["A branch, worktree or pull request already on a task", "feat/audit-pagination", "Links to the task that has it"],
  ["A Jira key in the branch name or the prompt", "BILL-881", "Links to the task with that key"],
  ["Words that resemble a few open tasks", "“remove the old checkout flags”", "Claude asks you once which one, or none"],
];

export function Matching() {
  return (
    <section id="how" className="section" aria-labelledby="how-title">
      <Reveal>
        <h2 id="how-title">How a session finds its task</h2>
        <p className="section-lede">
          With the Claude Code plugin installed, Tars reads the first prompt of every session in a git repo and
          checks these in order. The first one that matches decides. The first three link without asking you
          anything.
        </p>
      </Reveal>
      <Reveal delay={0.08}>
        <ol className="signals">
          {signals.map(([what, example, result]) => (
            <li key={what}>
              <span className="signal-what">{what}</span>
              <code className="signal-example">{example}</code>
              <span className="signal-result">{result}</span>
            </li>
          ))}
        </ol>
      </Reveal>
    </section>
  );
}

const threads: { day: string; prompt: string; how: string; id: string }[] = [
  { day: "Mon", prompt: "BILL-881 export still times out for Acme", how: "Linked by Jira key. Branch fix/csv-export recorded.", id: "4be1c9d0" },
  { day: "Mon", prompt: "batch the account lookups in the exporter", how: "Linked by branch", id: "9d02e7b1" },
  { day: "Tue", prompt: "stream rows instead of buffering them", how: "Linked by branch", id: "1c88fa43" },
  { day: "Wed", prompt: "show progress while the export runs", how: "Linked by branch", id: "e5a7302c" },
];

export function Threads() {
  return (
    <section className="section split" aria-labelledby="threads-title">
      <Reveal className="split-copy">
        <h2 id="threads-title">One feature, every thread</h2>
        <p>
          Real features take more than one session. Tars keeps them as one task with every session linked to it.
          Once the first thread links, its branch is recorded on the task, so later threads on that branch link
          on their own.
        </p>
        <p>
          Any of them can be reopened with <code>claude --resume</code>. If two tasks turn out to be the same work,{" "}
          <code>tars merge 31 21</code> moves the sessions, reports and log of #31 into #21.
        </p>
      </Reveal>
      <Reveal className="split-visual" delay={0.1}>
        <div className="thread-task">
          <p className="thread-title">
            <span className="task-id">#12</span> Fix CSV export timeout for large accounts
          </p>
          <ol className="thread-list" aria-label="Sessions linked to #12, oldest first">
            {threads.map((t) => (
              <li key={t.id}>
                <span className="thread-day">{t.day}</span>
                <span className="thread-prompt">{t.prompt}</span>
                <span className="thread-how">{t.how}</span>
                <code className="thread-resume">claude --resume {t.id}</code>
              </li>
            ))}
          </ol>
        </div>
      </Reveal>
    </section>
  );
}

type Choice = "file" | "new" | "dismiss";

const unfiled = [
  { id: "7c2e91", repo: "billing-svc", branch: "main", prompt: "bump the retry jitter to 20%", changes: "1 edit, 1 commit", task: 15 },
  { id: "3f9a12", repo: "platform", branch: "spike-nats", prompt: "spin up nats locally and send it test messages", changes: "6 edits", task: 21 },
  { id: "b40d5e", repo: "web", branch: "fix-footer", prompt: "fix the typo in the footer", changes: "2 edits", task: 0 },
];

export function Unfiled() {
  const [picked, setPicked] = useState<Record<string, Choice>>({});
  // The row the visitor last acted on gets focus back after its buttons swap.
  const [acted, setActed] = useState("");
  const [said, setSaid] = useState("");
  const choose = (id: string, c: Choice | null, msg: string) => {
    setActed(id);
    setSaid(msg);
    setPicked((p) => {
      const next = { ...p };
      if (c) next[id] = c;
      else delete next[id];
      return next;
    });
  };

  return (
    <section id="unfiled" className="section split" aria-labelledby="unfiled-title">
      <Reveal className="split-copy">
        <h2 id="unfiled-title">Nothing lands in your list uninvited</h2>
        <p>
          Capture links sessions to tasks. It never creates, edits or closes a task on its own.
        </p>
        <p>
          A session that changed code but matched no task waits under <strong>Unfiled</strong>. You file it into a
          task, make it a new task, or dismiss it. Sessions that only read code or answered a question leave
          nothing behind.
        </p>
        <p className="split-note">Try it on the right. The same choices are in the Mac app, in <code>/tars:tars</code>, and on the command line.</p>
      </Reveal>
      <Reveal className="split-visual" delay={0.1}>
        <p className="visually-hidden" aria-live="polite">{said}</p>
        <ul className="unfiled" aria-label="Unfiled sessions">
          {unfiled.map((s, i) => {
            const c = picked[s.id];
            const cmd =
              c === "file" ? `tars file ${s.id} ${s.task}` : c === "new" ? `tars file ${s.id} --new` : `tars dismiss ${s.id}`;
            const done =
              c === "file" ? `Filed into #${s.task}` : c === "new" ? `Filed as new task #${31 + i}` : "Dismissed";
            return (
              <li key={s.id} className={c ? "is-sorted" : undefined}>
                <p className="unfiled-prompt">{s.prompt}</p>
                <p className="unfiled-meta">
                  <code>{s.repo}</code> <code>{s.branch}</code> <span>{s.changes}</span>
                </p>
                <AnimatePresence mode="wait" initial={false}>
                  {c ? (
                    <motion.div
                      key="done"
                      className="unfiled-done"
                      initial={{ opacity: 0, y: 4 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -4 }}
                      transition={{ duration: 0.18 }}
                    >
                      <span className={c === "dismiss" ? "unfiled-status is-dismissed" : "unfiled-status"}>{done}</span>
                      <code className="unfiled-cmd">{cmd}</code>
                      <button type="button" className="text-btn" autoFocus={acted === s.id} onClick={() => choose(s.id, null, "Undone")}>
                        Undo
                      </button>
                    </motion.div>
                  ) : (
                    <motion.div
                      key="actions"
                      className="unfiled-actions"
                      initial={{ opacity: 0, y: 4 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -4 }}
                      transition={{ duration: 0.18 }}
                    >
                      {s.task > 0 && (
                        <button type="button" autoFocus={acted === s.id} onClick={() => choose(s.id, "file", `Filed into #${s.task}`)}>
                          File into #{s.task}
                        </button>
                      )}
                      <button type="button" autoFocus={acted === s.id && s.task === 0} onClick={() => choose(s.id, "new", `Filed as new task #${31 + i}`)}>
                        New task
                      </button>
                      <button type="button" onClick={() => choose(s.id, "dismiss", "Dismissed")}>
                        Dismiss
                      </button>
                    </motion.div>
                  )}
                </AnimatePresence>
              </li>
            );
          })}
        </ul>
      </Reveal>
    </section>
  );
}

export function Reports() {
  return (
    <section id="reports" className="section reports" aria-labelledby="reports-title">
      <Reveal>
        <h2 id="reports-title">Read what your agents did, not everything they said</h2>
        <p className="section-lede">
          When an agent finishes work on a task, it leaves a short report: what it did, how it checked, what it
          needs from you, and what comes next. Each field is capped at 240 characters, so a report takes seconds
          to read. The full session is still one <code>claude --resume</code> away.
        </p>
      </Reveal>
      <div className="compare">
        <Reveal className="compare-col">
          <p className="compare-label">In the terminal</p>
          <div className="transcript" aria-label="A long agent transcript, faded out">
            <p>I'll start by exploring the webhook sender to understand the current retry behaviour. Let me look at the delivery module first.</p>
            <p>I can see that deliveries are attempted once and any non-2xx response is treated as a permanent failure. Before changing this, let me check how the tests are structured and whether there's an existing backoff helper elsewhere in the codebase that I can reuse.</p>
            <p>I found a helper in pkg/retry, but it uses fixed intervals. I'll extend it to support exponential backoff with jitter, then wire it into the sender. Let me also make sure the alerting path is covered, since the ticket mentions alerting after the final failure.</p>
            <p>Now running the test suite to make sure nothing else depends on the old behaviour…</p>
            <p>All 212 tests pass, including 6 new ones. One thing I wasn't sure about: the ticket says to alert on-call, but there's also a #billing-alerts channel that the billing service already uses for…</p>
          </div>
        </Reveal>
        <Reveal className="compare-col" delay={0.12}>
          <p className="compare-label">In Tars</p>
          <div className="report-card">
            <p className="report-head">
              <span className="task-id">#15</span> Add retry with backoff to webhook sender
            </p>
            <dl>
              <dt>Done</dt>
              <dd>Exponential backoff, 5 tries. Committed a41c9e2 on tars/15-webhook-retry.</dd>
              <dt>Verified</dt>
              <dd>make test: 212 passed (6 new)</dd>
              <dt>Needs you</dt>
              <dd className="ask">Alert #oncall or #billing-alerts after the last retry?</dd>
              <dt>Next</dt>
              <dd>Wire the alert once you pick a channel</dd>
              <dt>Session</dt>
              <dd><code>claude --resume 8f3c21aa</code></dd>
            </dl>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

export function Surfaces() {
  return (
    <section id="surfaces" className="section" aria-labelledby="surfaces-title">
      <Reveal>
        <h2 id="surfaces-title">Read it wherever you are</h2>
        <p className="section-lede">
          The terminal list, Claude Code and the Mac app all read and write the same SQLite file on your machine.
          Below is the terminal list. It works with your keyboard.
        </p>
      </Reveal>
      <Reveal className="surfaces-demo" delay={0.08}>
        <TuiDemo />
      </Reveal>
      <dl className="surfaces">
        <Reveal delay={0.0}>
          <dt>In the terminal</dt>
          <dd>
            Run <code>tars</code> for the keyboard-driven list. Every command also takes <code>--json</code>, so
            scripts and agents can read the same data.
          </dd>
        </Reveal>
        <Reveal delay={0.05}>
          <dt>In Claude Code</dt>
          <dd>
            Ask in plain language: <code>/tars:tars what did my agents finish today?</code> Keep everything
            current with <code>/tars:update</code>.
          </dd>
        </Reveal>
        <Reveal delay={0.1}>
          <dt>On your Mac</dt>
          <dd>
            The Tars app shows All, Needs you, Active and Done, your Unfiled sessions, each task&rsquo;s reports
            and sessions, and a Repos page for per-repo preferences.
          </dd>
        </Reveal>
      </dl>
    </section>
  );
}

export function Profiles() {
  return (
    <section className="section split" aria-labelledby="profiles-title">
      <Reveal className="split-copy">
        <h2 id="profiles-title">Tell it once how you work</h2>
        <p>
          A repo profile holds what every agent in that repo should know: the definition of done, how to run the
          tests, the gotchas. Your personal profile holds how you like things done anywhere.
        </p>
        <p>
          Agents read both with <code>tars context</code> before they start. The repo&rsquo;s own CLAUDE.md still
          comes first.
        </p>
      </Reveal>
      <Reveal className="split-visual" delay={0.1}>
        <div className="profiles">
          <div className="profile">
            <p className="profile-name">billing-svc</p>
            <dl>
              <dt>dod</dt>
              <dd>Tests pass, migration reviewed, changelog entry added.</dd>
              <dt>tests</dt>
              <dd>
                <code>docker compose up db</code>, then <code>make test</code>.
              </dd>
              <dt>gotchas</dt>
              <dd>Exports read from the replica. Never write there.</dd>
            </dl>
          </div>
          <div className="profile">
            <p className="profile-name">You</p>
            <dl>
              <dt>style</dt>
              <dd>Small PRs. Plain commit messages.</dd>
              <dt>review</dt>
              <dd>Flag any change to a public API before making it.</dd>
            </dl>
          </div>
        </div>
      </Reveal>
    </section>
  );
}

export function Control() {
  return (
    <section className="section" aria-labelledby="control-title">
      <Reveal>
        <h2 id="control-title">You decide what&rsquo;s kept</h2>
      </Reveal>
      <dl className="rules">
        <Reveal delay={0.0}>
          <dt>Capture only links</dt>
          <dd>Capture files sessions under tasks. It never creates, edits or closes a task by itself.</dd>
        </Reveal>
        <Reveal delay={0.05}>
          <dt>Unmatched work waits</dt>
          <dd>Sessions that changed code and matched nothing sit in Unfiled until you sort them.</dd>
        </Reveal>
        <Reveal delay={0.1}>
          <dt>Turn capture off</dt>
          <dd>
            Everywhere with <code>tars capture off</code>, or for one repo with{" "}
            <code>tars capture off --repo billing-svc</code>.
          </dd>
        </Reveal>
        <Reveal delay={0.15}>
          <dt>Merge duplicates</dt>
          <dd>
            <code>tars merge 31 21</code> folds #31 into #21, with its sessions, reports and log.
          </dd>
        </Reveal>
        <Reveal delay={0.2}>
          <dt>Shows what&rsquo;s new</dt>
          <dd>
            A <span className="dot">●</span> marks agent work you haven&rsquo;t seen yet.
          </dd>
        </Reveal>
        <Reveal delay={0.25}>
          <dt>Keeps your data local</dt>
          <dd>Everything is in one SQLite file on your machine.</dd>
        </Reveal>
      </dl>
    </section>
  );
}
