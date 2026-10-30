import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { AnimatePresence, LayoutGroup, motion } from "motion/react";

type Status = "inbox" | "ready" | "in-progress" | "in-review" | "blocked" | "done";

type Task = {
  id: number;
  title: string;
  status: Status;
  priority: 0 | 1 | 2 | 3;
  jira?: string;
  repo?: string;
  when?: string;
  late?: boolean;
  waiting?: string;
  unread?: boolean;
  body?: string;
  criteria?: string[];
  branch?: string;
  report?: { done: string; verified?: string; needsYou?: string; next?: string; ago: string };
  session?: { name: string; id: string };
  log: { who: "you" | "agent"; kind: string; text: string; ago: string }[];
};

const initialTasks: Task[] = [
  {
    id: 12, title: "Fix CSV export timeout for large accounts", status: "in-progress", priority: 0,
    jira: "BILL-881", repo: "billing-svc", when: "2d late", late: true, branch: "tars/12-csv-export",
    body: "Exports over 200k rows time out after 30s. Acme hit it twice this week.",
    criteria: ["1M rows export in under 30s", "Progress is shown while exporting"],
    report: {
      done: "Batched account lookups; export of 1M rows now takes 41s.",
      verified: "make test: 198 passed",
      next: "Stream rows instead of buffering to get under 30s",
      ago: "3h ago",
    },
    session: { name: "csv export timeout", id: "4be1c9d0" },
    log: [
      { who: "you", kind: "note", text: "Root cause: N+1 query in the exporter.", ago: "yesterday" },
      { who: "agent", kind: "park", text: "Batched the account lookups. Next: stream rows instead of buffering.", ago: "3h ago" },
    ],
  },
  {
    id: 15, title: "Add retry with backoff to webhook sender", status: "blocked", priority: 1,
    jira: "BILL-902", repo: "billing-svc", when: "today",
    body: "Webhook deliveries fail permanently on a single 503.",
    criteria: ["Retries 5 times with exponential backoff", "Gives up and alerts after the last retry"],
    waiting: "you",
    report: {
      done: "Exponential backoff, 5 tries. Committed a41c9e2 on tars/15-webhook-retry.",
      verified: "make test: 212 passed (6 new)",
      needsYou: "Alert #oncall or #billing-alerts after the last retry?",
      ago: "40m ago",
    },
    session: { name: "webhook retry", id: "8f3c21aa" },
    log: [{ who: "agent", kind: "triage", text: "Pulled details from BILL-902 and drafted criteria. You confirmed them.", ago: "1d ago" }],
  },
  {
    id: 9, title: "Paginate the audit log API", status: "in-progress", priority: 2,
    jira: "WEB-77", repo: "web", when: "today",
    body: "The audit endpoint returns every row. Large tenants get 40MB responses.",
    criteria: ["Cursor-based pagination", "Page size defaults to 100"],
    log: [{ who: "agent", kind: "sweep", text: "Working in ../web.tars-9 …", ago: "1h ago" }],
  },
  {
    id: 18, title: "Ping Rahul about staging keys", status: "blocked", priority: 2,
    waiting: "Rahul", when: "Thu", log: [],
  },
  {
    id: 21, title: "Write the ADR for the event bus", status: "inbox", priority: 2,
    repo: "platform", when: "Nov 11", log: [],
  },
  { id: 23, title: "Clean up stale feature flags", status: "inbox", priority: 3, log: [] },
  { id: 7, title: "Upgrade Go to 1.27", status: "done", priority: 2, when: "2d ago", log: [] },
];

const needsYou = (t: Task) => !!t.report?.needsYou && t.status !== "done";

const views: { name: string; has: (t: Task) => boolean }[] = [
  { name: "Needs you", has: needsYou },
  { name: "Open", has: (t) => t.status !== "done" },
  { name: "Inbox", has: (t) => t.status === "inbox" },
  { name: "Ready", has: (t) => t.status === "ready" },
  { name: "Review", has: (t) => t.status === "in-review" },
  { name: "Blocked", has: (t) => t.status === "blocked" },
  { name: "Done", has: (t) => t.status === "done" },
];

const groups = ["Urgent", "High", "Normal", "Low"];
const statusIcon: Record<Status, string> = {
  inbox: "○", ready: "◇", "in-progress": "◐", "in-review": "◉", blocked: "■", done: "✔",
};
const statusName: Record<Status, string> = {
  inbox: "Inbox", ready: "Ready", "in-progress": "In progress", "in-review": "In review", blocked: "Blocked", done: "Done",
};
const prioMark = ["▲▲", "▲", "", "▽"];
const prioName = ["urgent", "high", "normal", "low"];

const AGENT_DELAY_MS = 4000;

// Motion presets for the demo: a quick glide for moving highlights and rows,
// a springy pop for dots, and a short crossfade between list and detail.
const glide = { type: "spring", stiffness: 520, damping: 42, mass: 0.6 } as const;
const spring = { type: "spring", stiffness: 600, damping: 20 } as const;
const swap = {
  initial: { opacity: 0, y: 4 },
  animate: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -4 },
  transition: { duration: 0.16 },
} as const;

export function TuiDemo() {
  const [tasks, setTasks] = useState(initialTasks);
  const [view, setView] = useState(1); // Open; "Needs you" sits first in the bar
  const [cursor, setCursor] = useState(0);
  const [open, setOpen] = useState(false);
  const [showLog, setShowLog] = useState(false);
  const [flash, setFlash] = useState("");
  const [undo, setUndo] = useState<Task[] | null>(null);
  const [agentDone, setAgentDone] = useState(false);
  const screenRef = useRef<HTMLDivElement>(null);

  // The one scripted moment: the agent finishes #9 while the visitor watches,
  // exactly the way sweep results show up in the real list.
  useEffect(() => {
    const timer = window.setTimeout(() => {
      setTasks((ts) =>
        ts.map((t): Task =>
          t.id === 9
            ? {
                ...t, status: "in-review", unread: true,
                report: {
                  done: "Cursor pagination, page size 100. Committed 7be03d1 on tars/9-audit-pagination.",
                  verified: "npm test: 388 passed (14 new)",
                  next: "Ready for your review",
                  ago: "just now",
                },
                session: { name: "audit log pagination", id: "c71d09e5" },
                log: [...t.log, { who: "agent", kind: "report", text: "Reported: cursor pagination, 388 tests passed.", ago: "just now" }],
              }
            : t,
        ),
      );
      setAgentDone(true);
    }, AGENT_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, []);

  const visible = useMemo(
    () =>
      tasks
        .filter(views[view].has)
        .sort((a, b) => a.priority - b.priority),
    [tasks, view],
  );
  const selected = visible[Math.min(cursor, visible.length - 1)];

  const say = (msg: string) => setFlash(msg);

  const markSeen = (id: number) =>
    setTasks((ts) => ts.map((t) => (t.id === id ? { ...t, unread: false } : t)));

  const change = (id: number, patch: Partial<Task>, msg: string) => {
    setUndo(tasks);
    setTasks((ts) => ts.map((t) => (t.id === id ? { ...t, ...patch } : t)));
    say(msg);
  };

  const switchView = (i: number) => {
    setView((i + views.length) % views.length);
    setCursor(0);
    setOpen(false);
  };

  const onKey = (e: KeyboardEvent) => {
    // Tab is left alone so keyboard users can always move focus out of the demo.
    if (e.metaKey || e.ctrlKey || e.altKey || e.key === "Tab") return;
    const key = e.key;
    const handled = act(key, e.shiftKey);
    if (handled) e.preventDefault();
  };

  // act runs one keystroke; it's shared by the keyboard and the on-screen keys.
  const act = (key: string, shift = false): boolean => {
    setFlash("");
    switch (key) {
      case "j":
      case "ArrowDown":
        setCursor((c) => Math.min(c + 1, visible.length - 1));
        if (open && visible[cursor + 1]) markSeen(visible[cursor + 1].id);
        return true;
      case "k":
      case "ArrowUp":
        setCursor((c) => Math.max(c - 1, 0));
        if (open && visible[cursor - 1]) markSeen(visible[cursor - 1].id);
        return true;
      case "Enter":
        if (!selected) return true;
        if (!open) markSeen(selected.id);
        setOpen((o) => !o);
        return true;
      case "Escape":
      case "h":
      case "ArrowLeft":
        setOpen(false);
        return true;
      case "Tab":
        switchView(view + (shift ? -1 : 1));
        return true;
      case "l":
        setShowLog((s) => !s);
        if (!open && selected) {
          markSeen(selected.id);
          setOpen(true);
        }
        return true;
      case "x":
        if (!selected) return true;
        if (selected.status === "done") change(selected.id, { status: "inbox" }, `#${selected.id} → Inbox · u to undo`);
        else change(selected.id, { status: "done", waiting: undefined }, `#${selected.id} → Done · u to undo`);
        setOpen(false);
        setCursor((c) => Math.max(0, Math.min(c, visible.length - 2)));
        return true;
      case "u":
        if (undo) {
          setTasks(undo);
          setUndo(null);
          say("Undid the last change");
        } else say("Nothing to undo");
        return true;
    }
    if (/^[1-7]$/.test(key)) {
      switchView(Number(key) - 1);
      return true;
    }
    return false;
  };

  const press = (key: string) => {
    act(key);
    screenRef.current?.focus();
  };

  // Rows with group headers, mirroring the real TUI's priority grouping.
  const rows: ({ header: string; count: number } | Task)[] = [];
  visible.forEach((t, i) => {
    if (i === 0 || visible[i - 1].priority !== t.priority) {
      rows.push({ header: groups[t.priority], count: visible.filter((x) => x.priority === t.priority).length });
    }
    rows.push(t);
  });

  const counts = views.map((v) => tasks.filter(v.has).length);
  const unread = views.map((v, i) => i !== views.length - 1 && tasks.some((t) => v.has(t) && t.unread));

  return (
    <figure className="tui-wrap">
      <div className="tui-window">
      <div className="tui-titlebar" aria-hidden="true">~/work · tars</div>
      <div
        ref={screenRef}
        className="tui"
        tabIndex={0}
        role="application"
        aria-label="Interactive Tars task list demo"
        aria-describedby="tui-help"
        onKeyDown={onKey}
      >
        <LayoutGroup id="tui">
        <div className="tui-top">
          <span className="tui-brand">TARS</span>
          {views.map((v, i) => (
            <button
              key={v.name}
              type="button"
              tabIndex={-1}
              className={i === view ? "tui-tab on" : "tui-tab"}
              onClick={() => { switchView(i); screenRef.current?.focus(); }}
            >
              {v.name} {counts[i]}
              <span className="tui-unread">
                <AnimatePresence>
                  {unread[i] && (
                    <motion.span
                      key="dot"
                      style={{ display: "inline-block" }}
                      initial={{ scale: 0, opacity: 0 }}
                      animate={{ scale: 1, opacity: 1 }}
                      exit={{ scale: 0, opacity: 0 }}
                      transition={spring}
                    >
                      ●
                    </motion.span>
                  )}
                </AnimatePresence>
              </span>
              {i === view && <motion.span layoutId="tabline" className="tui-tabline" transition={glide} />}
            </button>
          ))}
        </div>
        <div className="tui-rule" />

        <div className="tui-body">
          <AnimatePresence mode="wait" initial={false}>
            {open && selected ? (
              <motion.div key={`detail-${selected.id}`} {...swap}>
                <Detail task={selected} showLog={showLog} />
              </motion.div>
            ) : rows.length === 0 ? (
              <motion.p key={`empty-${view}`} className="tui-empty" {...swap}>
                Nothing here. Press 1–6 to see another view.
              </motion.p>
            ) : (
              <motion.ul key={`list-${view}`} className="tui-list" {...swap}>
                <AnimatePresence initial={false}>
                  {rows.map((r, i) =>
                    "header" in r ? (
                      <motion.li
                        layout="position"
                        key={`h-${r.header}`}
                        className={i === 0 ? "tui-group first" : "tui-group"}
                        exit={{ opacity: 0 }}
                        transition={glide}
                      >
                        {r.header.toUpperCase()} <span className="tui-muted">· {r.count}</span>
                      </motion.li>
                    ) : (
                      <motion.li
                        layout="position"
                        key={r.id}
                        className={[
                          "tui-row",
                          r === selected ? "sel" : "",
                          r.unread ? "unread" : "",
                          r.status === "done" ? "done" : "",
                        ].join(" ")}
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0, x: 12, transition: { duration: 0.18 } }}
                        transition={glide}
                        onClick={() => {
                          setCursor(visible.indexOf(r));
                          screenRef.current?.focus();
                        }}
                        onDoubleClick={() => {
                          setCursor(visible.indexOf(r));
                          markSeen(r.id);
                          setOpen(true);
                        }}
                      >
                        {r === selected && <motion.span layoutId="selection" className="tui-selbg" transition={glide} />}
                        <span className="c-bar">{r === selected ? "▌" : ""}</span>
                        <span className={needsYou(r) ? "c-dot c-ask" : "c-dot"}>
                          {needsYou(r) && "?"}
                          <AnimatePresence>
                            {r.unread && !needsYou(r) && (
                              <motion.span
                                key="dot"
                                style={{ display: "inline-block" }}
                                initial={{ scale: 0 }}
                                animate={{ scale: [0, 1.5, 1] }}
                                exit={{ scale: 0 }}
                                transition={{ duration: 0.5, ease: "easeOut" }}
                              >
                                ●
                              </motion.span>
                            )}
                          </AnimatePresence>
                        </span>
                        <span className={`c-prio p${r.priority}`}>{prioMark[r.priority]}</span>
                        <span className={`c-status s-${r.status}`}>{statusIcon[r.status]}</span>
                        <span className="c-id">#{r.id}</span>
                        <span className="c-title">
                          {r.title}
                          {r.status === "blocked" && r.waiting && <span className="c-waiting"> waiting on {r.waiting}</span>}
                        </span>
                        <span className="c-jira">{r.jira}</span>
                        <span className="c-repo">{r.repo}</span>
                        <span className={r.late ? "c-when late" : r.when === "today" ? "c-when today" : "c-when"}>{r.when}</span>
                      </motion.li>
                    ),
                  )}
                </AnimatePresence>
              </motion.ul>
            )}
          </AnimatePresence>
        </div>
        </LayoutGroup>

        <div className="tui-foot" aria-live="polite">
          {flash ? (
            <motion.span
              key={flash}
              className="tui-flash"
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.2 }}
            >
              ✔ {flash}
            </motion.span>
          ) : open ? (
            <Hints keys={[["j/k", "next/prev"], ["l", "activity"], ["x", "done"], ["esc", "back"]]} />
          ) : (
            <Hints keys={[["↑↓", "move"], ["⏎", "open"], ["x", "done"], ["u", "undo"], ["1–7", "view"]]} />
          )}
        </div>
      </div>
      </div>

      <figcaption id="tui-help" className="tui-caption">
        <span>
          {agentDone
            ? "The agent just finished #9 and left a report (see the ●). Select it and press enter to read it."
            : "This works like the real list. Click it, then press j, k, enter, x, u, or 1–7 to switch views. #15 is waiting on you (?)."}
        </span>
        <span className="tui-keys" aria-label="On-screen keys">
          {[["k", "↑"], ["j", "↓"], ["Enter", "enter"], ["x", "x"], ["u", "u"], ["Tab", "tab"]].map(([k, label]) => (
            <motion.button key={k} type="button" onClick={() => press(k)} whileTap={{ y: 2, scale: 0.96 }} transition={spring}>
              {label}
            </motion.button>
          ))}
        </span>
      </figcaption>
    </figure>
  );
}

function Hints({ keys }: { keys: [string, string][] }) {
  return (
    <span>
      {keys.map(([k, v]) => (
        <span key={k} className="tui-hint">
          <span className="tui-key">{k}</span> {v}
        </span>
      ))}
    </span>
  );
}

function Detail({ task, showLog }: { task: Task; showLog: boolean }) {
  const last = task.log[task.log.length - 1];
  return (
    <div className="tui-detail">
      <p className="tui-muted">
        #{task.id} <span className={`s-${task.status}`}>{statusIcon[task.status]} {statusName[task.status]}</span>
        {task.priority !== 2 && <span className={`p${task.priority}`}> {prioMark[task.priority]} {prioName[task.priority]}</span>}
      </p>
      <p className="tui-dtitle">{task.title}</p>
      {task.report && (
        <>
          <p className="tui-section">REPORT <span className="tui-muted">{task.report.ago}</span></p>
          <dl className="tui-fields tui-report">
            <dt>done</dt><dd>{task.report.done}</dd>
            {task.report.verified && (<><dt>verified</dt><dd>{task.report.verified}</dd></>)}
            {task.report.needsYou && (<><dt>needs you</dt><dd className="c-ask">{task.report.needsYou}</dd></>)}
            {task.report.next && (<><dt>next</dt><dd>{task.report.next}</dd></>)}
          </dl>
          {task.report.needsYou && <p className="tui-muted tui-indent-label">press <span className="tui-key">a</span> to answer</p>}
          <p className="tui-gap" />
        </>
      )}
      <dl className="tui-fields">
        {task.jira && (<><dt>jira</dt><dd className="c-jira">{task.jira}</dd></>)}
        {task.repo && (<><dt>repo</dt><dd>{task.repo}</dd></>)}
        {task.when && (<><dt>due</dt><dd className={task.late ? "late" : ""}>{task.when}</dd></>)}
        {task.waiting && (<><dt>waiting</dt><dd>{task.waiting}</dd></>)}
        {task.branch && (<><dt>branch</dt><dd>{task.branch}</dd></>)}
      </dl>
      {task.body && (<><p className="tui-section">DESCRIPTION</p><p>{task.body}</p></>)}
      {task.criteria && (
        <>
          <p className="tui-section">ACCEPTANCE CRITERIA</p>
          {task.criteria.map((c) => <p key={c}>- {c}</p>)}
        </>
      )}
      {task.session && (
        <>
          <p className="tui-section tui-gap">SESSIONS <span className="tui-key">o</span> <span className="tui-muted">copies resume command</span></p>
          <p>{task.session.name}</p>
          <p className="tui-muted tui-indent">claude --resume {task.session.id}…</p>
        </>
      )}
      {task.log.length === 0 ? (
        <p className="tui-muted tui-gap">No activity yet</p>
      ) : showLog ? (
        <>
          <p className="tui-section tui-gap">▾ ACTIVITY <span className="tui-key">l</span> <span className="tui-muted">collapse</span></p>
          {[...task.log].reverse().map((e) => (
            <div key={e.text} className="tui-entry">
              <p><span className={e.who === "agent" ? "tui-agent" : "tui-muted"}>{e.who}</span> <span className="tui-muted">· {e.kind} · {e.ago}</span></p>
              <p className="tui-indent">{e.text}</p>
            </div>
          ))}
        </>
      ) : (
        <p className="tui-gap">
          <span className="tui-section">▸ ACTIVITY</span>
          <span className="tui-muted"> · {task.log.length} {task.log.length === 1 ? "entry" : "entries"} · last: {last.who} · {last.kind} · {last.ago} </span>
          <span className="tui-key">l</span> <span className="tui-muted">expand</span>
        </p>
      )}
    </div>
  );
}
