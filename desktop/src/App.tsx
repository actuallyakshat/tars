import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, asCliError, isFatal, type CliError, type CliStatus, type NewTask, type Repo, type Status, type Task, type TaskEdit, type TaskSession } from "./api";
import {
  DONE,
  GROUPINGS,
  OPEN,
  PRIORITIES,
  STATUS,
  VIEWS,
  counts,
  defaultView,
  flatten,
  groupTasks,
  jiraUrl,
  openQuestion,
  resumeCommand,
  type GroupBy,
  type SortBy,
} from "./model";
import { Sidebar } from "./components/Sidebar";
import { TaskList } from "./components/TaskList";
import { Detail } from "./components/Detail";
import { CliProblem, ConfirmDialog } from "./components/Dialogs";
import { ContextMenu } from "./components/ContextMenu";
import { EditTaskPage, LegendPage, NewTaskPage, SettingsPage, type OpenMode } from "./components/Pages";
import { ReposPage } from "./components/ReposPage";
import type { PageName } from "./components/Sidebar";
import { CommandPalette, type Command } from "./components/CommandPalette";
import { PanesContext } from "./components/Panes";

export const REFRESH_MS = 4000;
export const RETRY_MS = 5000;
const JIRA_KEY = "tars.jiraBase";
const SIDEBAR_KEY = "tars.sidebarOpen";
const DETAIL_KEY = "tars.detailOpen";
const WIDTH_KEY = "tars.detailWidth";
const SIDEBAR_WIDTH_KEY = "tars.sidebarWidth";
const ZOOM_KEY = "tars.zoom";
const OPEN_MODE_KEY = "tars.openMode";
const GROUP_KEY = "tars.groupBy";
const SORT_KEY = "tars.sortBy";
const ZOOM_STEPS = [0.7, 0.8, 0.9, 1, 1.1, 1.25, 1.4, 1.5];
export const DETAIL_MIN = 340;
const DETAIL_DEFAULT = 460;
const LIST_MIN = 360;
// The sidebar resizes too, but only so far: past this it is wasted space.
export const SIDEBAR_MIN = 180;
export const SIDEBAR_MAX = 320;
const SIDEBAR_DEFAULT = 208;
const clampSidebar = (w: number) => Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, w));

const stored = (k: string, fallback: boolean) => {
  const v = localStorage.getItem(k);
  return v === null ? fallback : v === "1";
};

type Flash = { text: string; error: boolean } | null;

// The last status or priority change, so u can put it back.
type Undo = { id: number; what: string; run: () => Promise<unknown> } | null;

export function App() {
  const [cli, setCli] = useState<CliStatus | null>(null);
  const [fatal, setFatal] = useState<CliError | null>(null);
  const [bootError, setBootError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [view, setView] = useState(OPEN);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [detail, setDetail] = useState<Task | null>(null);
  const [focus, setFocus] = useState<"list" | "detail">("list");
  const [answering, setAnswering] = useState(false);
  const [noting, setNoting] = useState(false);
  const [logOpen, setLogOpen] = useState(false);
  const [page, setPage] = useState<PageName>("list");
  const [openMode, setOpenMode] = useState<OpenMode>(() => (localStorage.getItem(OPEN_MODE_KEY) === "full" ? "full" : "pane"));
  const [sidebarOpen, setSidebarOpen] = useState(() => stored(SIDEBAR_KEY, true));
  const [detailOpen, setDetailOpen] = useState(() => stored(DETAIL_KEY, true));
  const [detailWidth, setDetailWidth] = useState(() => Number(localStorage.getItem(WIDTH_KEY)) || DETAIL_DEFAULT);
  const [sidebarWidth, setSidebarWidth] = useState(() =>
    clampSidebar(Number(localStorage.getItem(SIDEBAR_WIDTH_KEY)) || SIDEBAR_DEFAULT),
  );
  // The selected task's details fill the window: no sidebar, no list.
  const [fullDetail, setFullDetail] = useState(false);
  const [flash, setFlash] = useState<Flash>(null);
  const [jiraBase, setJiraBase] = useState(() => localStorage.getItem(JIRA_KEY) ?? "");
  const [now, setNow] = useState(() => new Date());
  const [editing, setEditing] = useState<Task | null>(null);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [profiles, setProfiles] = useState<Repo[]>([]);
  const [zoom, setZoom] = useState(() => Number(localStorage.getItem(ZOOM_KEY)) || 1);
  const [query, setQuery] = useState("");
  const [groupBy, setGroupBy] = useState<GroupBy>(() => (localStorage.getItem(GROUP_KEY) as GroupBy) || "priority");
  const [sortBy, setSortBy] = useState<SortBy>(() => (localStorage.getItem(SORT_KEY) as SortBy) || "priority");
  const [undo, setUndo] = useState<Undo>(null);
  const [menu, setMenu] = useState<{ id: number; x: number; y: number } | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<Task | null>(null);

  const listOpts = { group: groupBy, sort: sortBy, query, now };
  const groups = useMemo(() => groupTasks(tasks, view, { group: groupBy, sort: sortBy, query, now }), [tasks, view, groupBy, sortBy, query, now]);
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

  // quiet: an automatic retry, which keeps the current error on screen
  // instead of flashing "Loading…".
  const boot = useCallback(async (quiet = false) => {
    if (!quiet) {
      setFatal(null);
      setBootError(null);
    }
    let list: Task[];
    try {
      setCli(await api.status());
      list = await api.list();
    } catch (e) {
      const err = asCliError(e);
      if (isFatal(err)) {
        setFatal(err);
        setBootError(null);
      } else {
        setFatal(null);
        setBootError(err.message);
      }
      setAttempt((n) => n + 1);
      return;
    }
    setFatal(null);
    setBootError(null);
    setTasks(list);
    setNow(new Date());
    const v = defaultView(list);
    setView(v);
    setSelectedId(flatten(groupTasks(list, v, { group: groupBy, sort: sortBy }))[0]?.id ?? null);
    indexRef.current = 0;
    setLoaded(true);
    api.repos().then(setProfiles, () => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    void boot();
  }, [boot]);

  useEffect(() => {
    if (!loaded || fatal) return;
    const h = setInterval(() => void refresh(), REFRESH_MS);
    return () => clearInterval(h);
  }, [loaded, fatal, refresh]);

  // Heal without a click: a failed load, or a missing or outdated CLI, is
  // tried again every few seconds, so fixing it (or the CLI repairing its
  // database) brings the list back on its own.
  useEffect(() => {
    if (!bootError && !fatal) return;
    const h = setTimeout(() => void boot(true), RETRY_MS);
    return () => clearTimeout(h);
  }, [bootError, fatal, attempt, boot]);

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
      },
      (e) => fail(e),
    );
  }, [selectedId, fail]);

  // Looking at a task is what "seen" means: clear the ● once its details are
  // on screen. Just moving the selection through the list doesn't count.
  const taskShowing = page === "list" && detailOpen && (openMode === "pane" || fullDetail);
  const seenId = detail && detail.id === selectedId && detail.unread && taskShowing ? detail.id : null;
  useEffect(() => {
    if (seenId === null) return;
    const id = seenId;
    api.seen(id).then(() => {
      setTasks((ts) => ts.map((t) => (t.id === id ? { ...t, unread: false } : t)));
      setDetail((d) => (d && d.id === id ? { ...d, unread: false } : d));
    }, fail);
  }, [seenId, fail]);

  // Agents add repos and preferences while the app is open; show the latest.
  useEffect(() => {
    if (page === "repos") api.repos().then(setProfiles, fail);
  }, [page, fail]);

  useEffect(() => {
    if (!flash) return;
    const h = setTimeout(() => setFlash(null), flash.error ? 6000 : 3000);
    return () => clearTimeout(h);
  }, [flash]);

  // Re-render on window resize so the preview width is re-checked against it.
  const [, setWinWidth] = useState(() => window.innerWidth);
  useEffect(() => {
    const onResize = () => setWinWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  useEffect(() => localStorage.setItem(SIDEBAR_KEY, sidebarOpen ? "1" : "0"), [sidebarOpen]);
  useEffect(() => localStorage.setItem(DETAIL_KEY, detailOpen ? "1" : "0"), [detailOpen]);
  useEffect(() => localStorage.setItem(WIDTH_KEY, String(Math.round(detailWidth))), [detailWidth]);
  useEffect(() => localStorage.setItem(SIDEBAR_WIDTH_KEY, String(Math.round(sidebarWidth))), [sidebarWidth]);
  useEffect(() => localStorage.setItem(GROUP_KEY, groupBy), [groupBy]);
  useEffect(() => localStorage.setItem(OPEN_MODE_KEY, openMode), [openMode]);
  useEffect(() => localStorage.setItem(SORT_KEY, sortBy), [sortBy]);
  useEffect(() => {
    localStorage.setItem(ZOOM_KEY, String(zoom));
    api.zoom(zoom).catch(() => {});
  }, [zoom]);

  // Keep the preview within what the window can fit beside a usable list.
  const clampWidth = useCallback(
    (w: number) => {
      const max = Math.max(DETAIL_MIN, window.innerWidth - (sidebarOpen ? sidebarWidth : 0) - LIST_MIN);
      return Math.min(max, Math.max(DETAIL_MIN, w));
    },
    [sidebarOpen, sidebarWidth],
  );

  // Shows the selected task the way Settings says: beside the list, or full screen.
  const showTask = () => {
    setDetailOpen(true);
    if (openMode === "full") setFullDetail(true);
  };

  const openDetail = () => {
    showTask();
    setFocus("detail");
  };

  const changeView = (i: number) => {
    setPage("list");
    setView(i);
    setSelectedId(flatten(groupTasks(tasks, i, listOpts))[0]?.id ?? null);
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

  const changeStatus = async (s: Status, text?: string, target: Task | null = selected) => {
    if (!target || s === target.status) return;
    const id = target.id;
    const before = target.status;
    const ok = await act(() => api.setStatus(id, s), `${text ?? `#${id} → ${STATUS[s].label}`}. Press u to undo.`);
    if (ok) setUndo({ id, what: `#${id} back to ${STATUS[before].label}`, run: () => api.setStatus(id, before) });
  };

  const changePriority = async (p: number) => {
    if (!selected || p === selected.priority) return;
    const id = selected.id;
    const before = selected.priority;
    const name = (i: number) => PRIORITIES[Math.min(3, Math.max(0, i))].toLowerCase();
    const ok = await act(() => api.edit(id, { priority: name(p) }), `#${id} priority → ${PRIORITIES[p]}. Press u to undo.`);
    if (ok) setUndo({ id, what: `#${id} back to ${PRIORITIES[before] ?? "normal"}`, run: () => api.edit(id, { priority: name(before) }) });
  };

  const undoLast = () => {
    if (!undo) {
      setFlash({ text: "Nothing to undo", error: true });
      return;
    }
    setUndo(null);
    void act(undo.run, `Undone: ${undo.what}`);
  };

  const toggleDone = (t: Task | null = selected) => {
    if (!t) return;
    if (t.status === "done") void changeStatus("inbox", `Reopened #${t.id}`, t);
    else void changeStatus("done", `#${t.id} done`, t);
  };

  const deleteTask = async (t: Task) => {
    const ok = await act(() => api.remove(t.id), `Deleted #${t.id} ${t.title}`);
    setConfirmDelete(null);
    if (ok && undo?.id === t.id) setUndo(null);
  };

  const openMenu = (id: number, x: number, y: number) => {
    setSelectedId(id);
    setFocus("list");
    setMenu({ id, x, y });
  };

  const saveEdit = async (e: TaskEdit) => {
    if (!editing) return false;
    if (Object.keys(e).length === 0) return true;
    const id = editing.id;
    return act(() => api.edit(id, e), `Saved #${id}`);
  };

  const startEdit = (t: Task | null = shown) => {
    if (!t) return;
    // The loaded detail has every field; fall back to the list's copy.
    setEditing(detail?.id === t.id ? detail : t);
    setPage("edit");
  };

  // Shows a task wherever it lives: the current view if it's there, else Open or Done.
  const goToTask = (id: number) => {
    const t = tasks.find((x) => x.id === id);
    if (!t) return;
    const v = VIEWS[view].has(t) ? view : t.status === "done" ? DONE : OPEN;
    setPage("list");
    setView(v);
    setQuery("");
    setSelectedId(id);
    openDetail();
  };

  const setRepo = async (name: string, path: string) => {
    const ok = await act(() => api.setRepo(name, path), `Saved repo ${name}`);
    if (ok) api.repos().then(setProfiles, fail);
    return ok;
  };

  const removeRepo = async (name: string) => {
    const ok = await act(() => api.removeRepo(name), `Removed repo ${name}`);
    if (ok) api.repos().then(setProfiles, fail);
    return ok;
  };

  const chooseFolder = () =>
    api.chooseFolder().catch((e) => {
      fail(e);
      return null;
    });

  const zoomBy = (d: number) => {
    const i = ZOOM_STEPS.findIndex((z) => z >= zoom - 0.001);
    const next = d === 0 ? 1 : ZOOM_STEPS[Math.max(0, Math.min(ZOOM_STEPS.length - 1, (i < 0 ? 3 : i) + d))];
    setZoom(next);
    say(`Zoom ${Math.round(next * 100)}%`);
  };

  const toggleSidebar = () => setSidebarOpen((o) => !o);
  const toggleDetail = () => {
    setFocus("list");
    if (openMode === "full") {
      // There's no side pane in this mode: toggling means opening or leaving full screen.
      if (fullDetail) setFullDetail(false);
      else if (selected) openDetail();
      return;
    }
    setDetailOpen((o) => !o);
    setFullDetail(false);
  };
  const toggleFull = () => {
    if (!selected && !fullDetail) return;
    setFullDetail((f) => !f);
    setDetailOpen(true);
    setFocus("detail");
  };

  const answer = async (text: string) => {
    if (!selected) return false;
    const id = selected.id;
    const changing = !!shown?.report?.answered_at;
    const ok = await act(
      () => api.answer(id, text),
      changing ? `Changed your answer on #${id}. The next agent on it will see the new one.` : `Answered #${id}. The next agent on it will see your answer.`,
    );
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
    if (list && flatten(groupTasks(list, view, listOpts)).some((x) => x.id === created.id)) setSelectedId(created.id);
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
    if (!shown.needs_you && !openQuestion(shown) && !shown.report?.needs_you) {
      setFlash({ text: `#${shown.id} has no open question`, error: true });
      return;
    }
    openDetail();
    setAnswering(true);
  };

  const onKey = (e: KeyboardEvent) => {
    const k = e.key.toLowerCase();
    if (e.metaKey && !e.altKey && !e.ctrlKey && k === "k" && !fatal) {
      e.preventDefault();
      setPaletteOpen((o) => !o);
      return;
    }
    if (paletteOpen || menu || confirmDelete) return;
    if (e.metaKey && !e.altKey && !e.ctrlKey) {
      if (k === "=" || k === "+") {
        zoomBy(1);
      } else if (k === "-") {
        zoomBy(-1);
      } else if (k === "0") {
        zoomBy(0);
      } else if (k === "r") {
        if (fatal) void boot();
        else void refresh().then((l) => l && say("Refreshed"));
      } else if (k === "n" && !fatal) {
        setPage("new");
      } else if (k === "backspace") {
        // ⌘⌫ in a text field deletes text, not the task.
        const tag = (e.target as HTMLElement | null)?.tagName;
        if (tag === "INPUT" || tag === "TEXTAREA" || page !== "list" || !selected) return;
        setConfirmDelete(selected);
      } else if (k === "b" && e.shiftKey) {
        toggleDetail();
      } else if (k === "b") {
        toggleSidebar();
      } else {
        return;
      }
      e.preventDefault();
      return;
    }
    if (fatal || e.metaKey || e.ctrlKey || e.altKey) return;
    if (page !== "list") {
      if (e.key === "Escape") {
        e.preventDefault();
        setPage("list");
      }
      return;
    }
    const el = e.target as HTMLElement | null;
    const tag = el?.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || el?.isContentEditable) return;
    if (tag === "BUTTON" && (e.key === "Enter" || e.key === " ")) return;
    // Tab switches views only from the list itself; elsewhere it moves focus as usual.
    if (e.key === "Tab") {
      const inList = !el || el === document.body || !!el.closest(".list-scroll");
      if (!inList) return;
      e.preventDefault();
      changeView((view + (e.shiftKey ? VIEWS.length - 1 : 1)) % VIEWS.length);
      return;
    }
    switch (e.key) {
      case "j":
      case "ArrowDown":
        move(1);
        break;
      case "k":
      case "ArrowUp":
        move(-1);
        break;
      case "Home":
        if (visible.length) setSelectedId(visible[0].id);
        break;
      case "End":
        if (visible.length) setSelectedId(visible[visible.length - 1].id);
        break;
      case "Enter":
        if (selected) openDetail();
        break;
      case "Escape":
        // Peel back one layer at a time: a draft, then focus, then the preview,
        // then the filter.
        if (answering || noting) {
          setAnswering(false);
          setNoting(false);
        } else if (fullDetail) setFullDetail(false);
        else if (focus === "detail") setFocus("list");
        else if (detailOpen) setDetailOpen(false);
        else if (query) setQuery("");
        else return;
        break;
      case "/":
        document.getElementById("list-search")?.focus();
        break;
      case "g": {
        const i = GROUPINGS.findIndex((g) => g.id === groupBy);
        const next = GROUPINGS[(i + 1) % GROUPINGS.length];
        setGroupBy(next.id);
        say(next.id === "none" ? "Grouping off" : `Grouped by ${next.label.toLowerCase()}`);
        break;
      }
      case "e":
        if (!selected) return;
        startEdit();
        break;
      case "p":
        if (!selected) return;
        showTask();
        requestAnimationFrame(() => document.getElementById("priority-select")?.click());
        break;
      case "u":
        undoLast();
        break;
      case "x":
        toggleDone(selected);
        break;
      case "a":
        startAnswer();
        break;
      case "n":
        if (!selected) return;
        openDetail();
        setNoting(true);
        break;
      case "c":
        setPage("new");
        break;
      case "?":
        setPage("legend");
        break;
      case "s":
        if (!selected) return;
        showTask();
        requestAnimationFrame(() => document.getElementById("status-select")?.click());
        break;
      case "l":
        setLogOpen((v) => !v);
        break;
      case "f":
        if (!selected && !fullDetail) return;
        toggleFull();
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

  const repos = [...new Set([...profiles.map((r) => r.name), ...tasks.map((t) => t.repo).filter((r): r is string => !!r)])].sort();
  const commands: Command[] = [
    { id: "new", label: "New task", keys: "⌘N", run: () => setPage("new") },
    ...(shown
      ? [
          { id: "edit", label: `Edit #${shown.id}`, keys: "E", run: () => startEdit(shown) },
          {
            id: "full",
            label: fullDetail ? "Exit full screen" : `Open #${shown.id} full screen`,
            keys: "F",
            run: () => {
              setPage("list");
              toggleFull();
            },
          },
          { id: "delete", label: `Delete #${shown.id}…`, keys: "⌘⌫", run: () => setConfirmDelete(shown) },
        ]
      : []),
    ...VIEWS.map((v, i) => ({ id: `view-${i}`, label: `Go to ${v.name}`, keys: String(i + 1), run: () => changeView(i) })),
    ...GROUPINGS.map((g) => ({
      id: `group-${g.id}`,
      label: g.id === "none" ? "Turn grouping off" : `Group by ${g.label.toLowerCase()}`,
      run: () => {
        setPage("list");
        setGroupBy(g.id);
      },
    })),
    { id: "sidebar", label: sidebarOpen ? "Hide sidebar" : "Show sidebar", keys: "⌘B", run: toggleSidebar },
    { id: "preview", label: detailOpen ? "Hide preview" : "Show preview", keys: "⌘⇧B", run: toggleDetail },
    { id: "repos", label: "Repos and preferences", run: () => setPage("repos") },
    { id: "settings", label: "Settings", run: () => setPage("settings") },
    {
      id: "open-mode",
      label: openMode === "full" ? "Open tasks in the side pane" : "Open tasks full screen",
      run: () => setOpenMode(openMode === "full" ? "pane" : "full"),
    },
    { id: "legend", label: "Legend", keys: "?", run: () => setPage("legend") },
    { id: "refresh", label: "Refresh", keys: "⌘R", run: () => void refresh().then((l) => l && say("Refreshed")) },
    { id: "zoom-in", label: "Zoom in", keys: "⌘+", run: () => zoomBy(1) },
    { id: "zoom-out", label: "Zoom out", keys: "⌘−", run: () => zoomBy(-1) },
    { id: "zoom-reset", label: "Actual size", keys: "⌘0", run: () => zoomBy(0) },
  ];
  // In full-screen mode the task only shows full screen, never as a side pane.
  const showDetail = taskShowing;
  const openByRepo: Record<string, number> = {};
  for (const t of tasks) if (t.repo && t.status !== "done") openByRepo[t.repo] = (openByRepo[t.repo] ?? 0) + 1;
  const full = showDetail && fullDetail;
  const showSidebar = sidebarOpen && !full;
  const columns = full
    ? "minmax(0, 1fr)"
    : [sidebarOpen ? `${sidebarWidth}px` : null, "minmax(0, 1fr)", showDetail ? `${clampWidth(detailWidth)}px` : null]
        .filter(Boolean)
        .join(" ");

  return (
    <PanesContext.Provider value={{ sidebarOpen, detailOpen: showDetail, previewPane: openMode === "pane", toggleSidebar, toggleDetail }}>
    <div
      className={"app" + (focus === "detail" ? " detail-focus" : "") + (showSidebar ? "" : " no-sidebar") + (full ? " full-detail" : "")}
      style={{ gridTemplateColumns: columns }}
    >
      {showSidebar && (
        <div className="sidebar-wrap">
        <Sidebar
          counts={viewCounts}
          view={view}
          page={page}
          onView={changeView}
          onPage={(p) => setPage(p)}
        />
        <Resizer
          label="Resize sidebar"
          edge="right"
          width={sidebarWidth}
          onWidth={(w) => setSidebarWidth(clampSidebar(w))}
          onReset={() => setSidebarWidth(SIDEBAR_DEFAULT)}
        />
        </div>
      )}
      {page === "new" && <NewTaskPage repos={repos} onAdd={add} onBack={() => setPage("list")} />}
      {page === "edit" && editing && (
        <EditTaskPage key={editing.id} task={editing} repos={repos} onSave={saveEdit} onBack={() => setPage("list")} />
      )}
      {page === "settings" && (
        <SettingsPage
          jiraBase={jiraBase}
          cli={cli}
          openMode={openMode}
          onOpenMode={setOpenMode}
          onSaveJira={(b) => {
            setJiraBase(b);
            localStorage.setItem(JIRA_KEY, b);
          }}
          onBack={() => setPage("list")}
        />
      )}
      {page === "repos" && (
        <ReposPage
          repos={profiles}
          openTasks={openByRepo}
          onLoadRepo={(name) => api.showRepo(name).catch((e) => (fail(e), null))}
          onLoadProfile={() => api.profile().catch((e) => (fail(e), null))}
          onSetRepo={setRepo}
          onRemoveRepo={removeRepo}
          onRenameRepo={async (from, to) => {
            const ok = await act(() => api.renameRepo(from, to), `Renamed ${from} to ${to.trim()}. Its tasks moved with it.`);
            if (ok) api.repos().then(setProfiles, fail);
            return ok;
          }}
          onSetSection={(repo, section, text) =>
            act(
              () => (repo === null ? api.setProfileSection(section, text) : api.setRepoSection(repo, section, text)),
              text.trim() ? "Saved preference" : "Removed preference",
            )
          }
          onChooseFolder={chooseFolder}
          onReveal={(p) => void api.reveal(p).catch(fail)}
          onBack={() => setPage("list")}
        />
      )}
      {page === "legend" && <LegendPage onBack={() => setPage("list")} />}
      {page === "list" && !full && (
        <TaskList
          groups={groups}
          view={view}
          totalOpen={viewCounts[OPEN].total}
          selectedId={selectedId}
          now={now}
          query={query}
          group={groupBy}
          sort={sortBy}
          onQuery={setQuery}
          onGroup={setGroupBy}
          onSort={setSortBy}
          onSelect={(id) => {
            setSelectedId(id);
            if (openMode === "full") openDetail();
            else {
              setDetailOpen(true);
              setFocus("list");
            }
          }}
          onOpen={(id) => {
            setSelectedId(id);
            openDetail();
          }}
          onMenu={openMenu}
          onOpenUrl={(url) => void api.openUrl(url).catch(fail)}
        />
      )}
      {showDetail && (
        <div className="detail-wrap">
          {!full && (
            <Resizer
              label="Resize preview"
              edge="left"
              width={clampWidth(detailWidth)}
              onWidth={(w) => setDetailWidth(clampWidth(w))}
              onReset={() => setDetailWidth(DETAIL_DEFAULT)}
            />
          )}
          <Detail
            task={shown}
            now={now}
            focused={focus === "detail"}
            full={full}
            onToggleFull={toggleFull}
            answering={answering}
            noting={noting}
            logOpen={logOpen}
            onClose={() => {
              setDetailOpen(false);
              setFullDetail(false);
              setFocus("list");
            }}
            onEdit={() => startEdit(shown)}
            onStatus={(s) => void changeStatus(s)}
            onPriority={(p) => void changePriority(p)}
            onToggleDone={() => toggleDone(selected)}
            onAnswer={answer}
            onNote={note}
            onJira={openJira}
            onOpenUrl={(url) => void api.openUrl(url).catch(fail)}
            onCopy={(text, what) => void copy(text, what)}
            onReveal={(p) => void api.reveal(p).catch(fail)}
            onTerminal={(s) => void openTerminal(s)}
            setAnswering={setAnswering}
            setNoting={setNoting}
            setLogOpen={setLogOpen}
          />
        </div>
      )}
      {flash && (
        <div className={"flash" + (flash.error ? " error" : "")} role={flash.error ? "alert" : "status"}>
          {flash.text}
        </div>
      )}
      {menu &&
        (() => {
          const t = tasks.find((x) => x.id === menu.id);
          if (!t) return null;
          return (
            <ContextMenu
              x={menu.x}
              y={menu.y}
              label={`#${t.id} actions`}
              onClose={() => setMenu(null)}
              items={[
                { label: "Edit…", keys: "E", run: () => startEdit(t) },
                {
                  label: "Open full screen",
                  keys: "F",
                  run: () => {
                    setSelectedId(t.id);
                    setFullDetail(true);
                    openDetail();
                  },
                },
                { label: t.status === "done" ? "Reopen" : "Mark done", keys: "X", run: () => toggleDone(t) },
                { label: "Delete…", keys: "⌘⌫", danger: true, run: () => setConfirmDelete(t) },
              ]}
            />
          );
        })()}
      {confirmDelete && (
        <ConfirmDialog
          title={`Delete #${confirmDelete.id}?`}
          body={`“${confirmDelete.title}” and its notes, agent reports and session links will be deleted for good. This can't be undone.`}
          confirmLabel="Delete task"
          onConfirm={() => deleteTask(confirmDelete)}
          onCancel={() => setConfirmDelete(null)}
        />
      )}
      {paletteOpen && <CommandPalette tasks={tasks} commands={commands} onTask={goToTask} onClose={() => setPaletteOpen(false)} />}
    </div>
    </PanesContext.Provider>
  );
}

// A pane divider. Drag to resize, double-click to reset, or focus it and use
// ← →. `edge` is the side of its pane it sits on: the preview's left edge grows
// the pane when dragged left, the sidebar's right edge when dragged right.
function Resizer({
  label,
  edge,
  width,
  onWidth,
  onReset,
}: {
  label: string;
  edge: "left" | "right";
  width: number;
  onWidth: (w: number) => void;
  onReset: () => void;
}) {
  const grow = edge === "left" ? -1 : 1;
  const start = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    const el = e.currentTarget;
    el.setPointerCapture(e.pointerId);
    const x0 = e.clientX;
    const w0 = width;
    const moveTo = (ev: PointerEvent) => onWidth(w0 + grow * (ev.clientX - x0));
    const stop = () => {
      el.removeEventListener("pointermove", moveTo);
      el.removeEventListener("pointerup", stop);
      document.body.classList.remove("resizing");
    };
    document.body.classList.add("resizing");
    el.addEventListener("pointermove", moveTo);
    el.addEventListener("pointerup", stop);
  };
  return (
    <div
      className={"resizer at-" + edge}
      role="separator"
      aria-orientation="vertical"
      aria-label={label}
      aria-valuenow={Math.round(width)}
      tabIndex={0}
      onPointerDown={start}
      onDoubleClick={onReset}
      onKeyDown={(e) => {
        if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
          e.preventDefault();
          e.stopPropagation();
          onWidth(width + grow * (e.key === "ArrowRight" ? 24 : -24));
        }
      }}
    />
  );
}
