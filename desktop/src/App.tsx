import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, asCliError, isFatal, type CliError, type CliStatus, type NewTask, type Status, type Task, type TaskSession } from "./api";
import { OPEN, STATUS, counts, defaultView, flatten, groupTasks, jiraUrl, openQuestion, resumeCommand } from "./model";
import { Sidebar } from "./components/Sidebar";
import { TaskList } from "./components/TaskList";
import { Detail } from "./components/Detail";
import { AddTask, CliProblem, Settings } from "./components/Dialogs";

export const REFRESH_MS = 4000;
const JIRA_KEY = "tars.jiraBase";

type Flash = { text: string; error: boolean } | null;

export function App() {
  const [cli, setCli] = useState<CliStatus | null>(null);
  const [fatal, setFatal] = useState<CliError | null>(null);
  const [bootError, setBootError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [view, setView] = useState(OPEN);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [detail, setDetail] = useState<Task | null>(null);
  const [focus, setFocus] = useState<"list" | "detail">("list");
  const [answering, setAnswering] = useState(false);
  const [noting, setNoting] = useState(false);
  const [logOpen, setLogOpen] = useState(false);
  const [dialog, setDialog] = useState<null | "add" | "settings">(null);
  const [flash, setFlash] = useState<Flash>(null);
  const [jiraBase, setJiraBase] = useState(() => localStorage.getItem(JIRA_KEY) ?? "");
  const [now, setNow] = useState(() => new Date());

  const groups = useMemo(() => groupTasks(tasks, view), [tasks, view]);
  const visible = useMemo(() => flatten(groups), [groups]);
  const viewCounts = useMemo(() => counts(tasks), [tasks]);
  const selected = visible.find((t) => t.id === selectedId) ?? null;
  const shown = selected && detail?.id === selected.id ? detail : selected;

  const selRef = useRef(selectedId);
  selRef.current = selectedId;
  const indexRef = useRef(0);

  const say = (text: string) => setFlash({ text, error: false });
  const fail = useCallback((e: unknown) => {
    const err = asCliError(e);
    if (isFatal(err)) setFatal(err);
    else setFlash({ text: err.message, error: true });
  }, []);

  // One refresh at a time; a request during one runs once it finishes.
  const busy = useRef(false);
  const again = useRef(false);
  const refresh = useCallback(async (): Promise<Task[] | null> => {
    if (busy.current) {
      again.current = true;
      return null;
    }
    busy.current = true;
    try {
      const list = await api.list();
      setTasks(list);
      setNow(new Date());
      const id = selRef.current;
      if (id !== null && list.some((t) => t.id === id)) {
        const full = await api.show(id);
        if (selRef.current === id) setDetail(full);
      }
      return list;
    } catch (e) {
      fail(e);
      return null;
    } finally {
      busy.current = false;
      if (again.current) {
        again.current = false;
        void refresh();
      }
    }
  }, [fail]);

  const boot = useCallback(async () => {
    setFatal(null);
    setBootError(null);
    let list: Task[];
    try {
      setCli(await api.status());
      list = await api.list();
    } catch (e) {
      const err = asCliError(e);
      if (isFatal(err)) setFatal(err);
      else setBootError(err.message);
      return;
    }
    setTasks(list);
    setNow(new Date());
    const v = defaultView(list);
    setView(v);
    setSelectedId(flatten(groupTasks(list, v))[0]?.id ?? null);
    indexRef.current = 0;
    setLoaded(true);
  }, []);

  useEffect(() => {
    void boot();
  }, [boot]);

  useEffect(() => {
    if (!loaded || fatal) return;
    const h = setInterval(() => void refresh(), REFRESH_MS);
    return () => clearInterval(h);
  }, [loaded, fatal, refresh]);

  // Keep the selection on the same task across refreshes; when it leaves the
  // view, take the task that moved into its place.
  useEffect(() => {
    if (!loaded) return;
    const i = visible.findIndex((t) => t.id === selectedId);
    if (i >= 0) {
      indexRef.current = i;
      return;
    }
    const next = visible[Math.min(indexRef.current, visible.length - 1)];
    setSelectedId(next ? next.id : null);
  }, [visible, selectedId, loaded]);

  useEffect(() => {
    setAnswering(false);
    setNoting(false);
    if (selectedId === null) return;
    const id = selectedId;
    api.show(id).then(
      (full) => {
        if (selRef.current !== id) return;
        setDetail(full);
        // Looking at a task is what "seen" means: clear the ● right away.
        if (full.unread) {
          api.seen(id).then(() => {
            setTasks((ts) => ts.map((t) => (t.id === id ? { ...t, unread: false } : t)));
            setDetail((d) => (d && d.id === id ? { ...d, unread: false } : d));
          }, fail);
        }
      },
      (e) => fail(e),
    );
  }, [selectedId, fail]);

  useEffect(() => {
    if (!flash) return;
    const h = setTimeout(() => setFlash(null), flash.error ? 6000 : 3000);
    return () => clearTimeout(h);
  }, [flash]);

  const changeView = (i: number) => {
    setView(i);
    setSelectedId(flatten(groupTasks(tasks, i))[0]?.id ?? null);
    indexRef.current = 0;
    setFocus("list");
  };

  const move = (d: number) => {
    if (visible.length === 0) return;
    const i = visible.findIndex((t) => t.id === selectedId);
    const next = visible[Math.max(0, Math.min(visible.length - 1, i + d))];
    setSelectedId(next.id);
  };

  const act = async (fn: () => Promise<unknown>, ok: string) => {
    try {
      await fn();
    } catch (e) {
      fail(e);
      return false;
    }
    say(ok);
    await refresh();
    return true;
  };

  const changeStatus = (s: Status) => {
    if (!selected || s === selected.status) return;
    const id = selected.id;
    void act(() => api.setStatus(id, s), `#${id} → ${STATUS[s].label}`);
  };

  const toggleDone = () => {
    if (!selected) return;
    const id = selected.id;
    if (selected.status === "done") void act(() => api.setStatus(id, "inbox"), `Reopened #${id}`);
    else void act(() => api.setStatus(id, "done"), `#${id} done`);
  };

  const answer = async (text: string) => {
    if (!selected) return false;
    const id = selected.id;
    const ok = await act(() => api.answer(id, text), `Answered #${id}. The next agent on it will see your answer.`);
    if (ok) setAnswering(false);
    return ok;
  };

  const note = async (text: string) => {
    if (!selected) return false;
    const id = selected.id;
    const ok = await act(() => api.note(id, text), `Noted on #${id}`);
    if (ok) setNoting(false);
    return ok;
  };

  const add = async (t: NewTask) => {
    let created: Task;
    try {
      created = await api.add(t);
    } catch (e) {
      fail(e);
      return false;
    }
    say(`Added #${created.id} ${created.title}`);
    const list = await refresh();
    if (list && flatten(groupTasks(list, view)).some((x) => x.id === created.id)) setSelectedId(created.id);
    return true;
  };

  const copy = (text: string, what: string) =>
    api.copy(text).then(() => say(`Copied ${what}`), fail);

  const openJira = (key: string) => {
    const url = jiraUrl(jiraBase, key);
    if (url) api.openUrl(url).catch(fail);
    else api.copy(key).then(() => say(`Copied ${key}. Add your Jira URL in Settings to open keys in the browser.`), fail);
  };

  const openTerminal = (s: TaskSession) =>
    api.openInTerminal(s.id, s.launch_dir).then(() => say("Opened in Terminal"), fail);

  const startAnswer = () => {
    if (!shown) return;
    if (!shown.needs_you && !openQuestion(shown)) {
      setFlash({ text: `#${shown.id} has no open question`, error: true });
      return;
    }
    setFocus("detail");
    setAnswering(true);
  };

  const onKey = (e: KeyboardEvent) => {
    if (e.metaKey && e.key.toLowerCase() === "r") {
      e.preventDefault();
      if (fatal) void boot();
      else void refresh().then((l) => l && say("Refreshed"));
      return;
    }
    if (fatal || dialog || e.metaKey || e.ctrlKey || e.altKey) return;
    const el = e.target as HTMLElement | null;
    const tag = el?.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || el?.isContentEditable) return;
    if (tag === "BUTTON" && (e.key === "Enter" || e.key === " ")) return;
    switch (e.key) {
      case "j":
      case "ArrowDown":
        move(1);
        break;
      case "k":
      case "ArrowUp":
        move(-1);
        break;
      case "Enter":
        if (selected) setFocus("detail");
        break;
      case "Escape":
        setFocus("list");
        setAnswering(false);
        setNoting(false);
        break;
      case "x":
        toggleDone();
        break;
      case "a":
        startAnswer();
        break;
      case "n":
        if (!selected) return;
        setFocus("detail");
        setNoting(true);
        break;
      case "c":
        setDialog("add");
        break;
      case "s":
        document.getElementById("status-select")?.focus();
        break;
      case "l":
        setLogOpen((v) => !v);
        break;
      case "o": {
        const s = shown?.sessions?.[0];
        if (s) void copy(resumeCommand(s.id), "resume command");
        break;
      }
      default:
        if (/^[1-7]$/.test(e.key)) changeView(Number(e.key) - 1);
        else return;
    }
    e.preventDefault();
  };
  const keyRef = useRef(onKey);
  keyRef.current = onKey;
  useEffect(() => {
    const h = (e: KeyboardEvent) => keyRef.current(e);
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, []);

  if (fatal) return <CliProblem error={fatal} onRetry={() => void boot()} />;
  if (!loaded) {
    return (
      <main className="loading" aria-busy={!bootError}>
        <div className="drag problem-top" data-tauri-drag-region />
        {!bootError ? (
          <p className="muted">Loading tasks…</p>
        ) : (
          <>
            <p role="alert">Couldn't load tasks: {bootError}</p>
            <button className="primary" onClick={() => void boot()}>
              Try again
            </button>
          </>
        )}
      </main>
    );
  }

  return (
    <div className={"app" + (focus === "detail" ? " detail-focus" : "")}>
      <Sidebar
        counts={viewCounts}
        view={view}
        onView={changeView}
        onAdd={() => setDialog("add")}
        onSettings={() => setDialog("settings")}
      />
      <TaskList
        groups={groups}
        view={view}
        totalOpen={viewCounts[OPEN].total}
        selectedId={selectedId}
        now={now}
        onSelect={(id) => {
          setSelectedId(id);
          setFocus("list");
        }}
        onOpen={(id) => {
          setSelectedId(id);
          setFocus("detail");
        }}
      />
      <Detail
        task={shown}
        now={now}
        focused={focus === "detail"}
        answering={answering}
        noting={noting}
        logOpen={logOpen}
        onStatus={changeStatus}
        onToggleDone={toggleDone}
        onAnswer={answer}
        onNote={note}
        onJira={openJira}
        onCopy={(text, what) => void copy(text, what)}
        onReveal={(p) => void api.reveal(p).catch(fail)}
        onTerminal={(s) => void openTerminal(s)}
        setAnswering={setAnswering}
        setNoting={setNoting}
        setLogOpen={setLogOpen}
      />
      {flash && (
        <div className={"flash" + (flash.error ? " error" : "")} role={flash.error ? "alert" : "status"}>
          {flash.text}
        </div>
      )}
      {dialog === "add" && <AddTask onAdd={add} onClose={() => setDialog(null)} />}
      {dialog === "settings" && (
        <Settings
          jiraBase={jiraBase}
          cli={cli}
          onSave={(b) => {
            setJiraBase(b);
            localStorage.setItem(JIRA_KEY, b);
          }}
          onClose={() => setDialog(null)}
        />
      )}
    </div>
  );
}
