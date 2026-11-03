// An in-memory stand-in for the Rust commands, so components run against
// something that behaves like the CLI.
import { invoke } from "@tauri-apps/api/core";
import { vi } from "vitest";
import type { Capture, CliError, Repo, Section, Task, TodoItem, TodoList, UsageLine, UsageSummary } from "../api";

const NOW = new Date();
const iso = (minutesAgo: number) => new Date(NOW.getTime() - minutesAgo * 60_000).toISOString();
const day = (offset: number) => {
  const d = new Date(NOW.getFullYear(), NOW.getMonth(), NOW.getDate() + offset);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

export function sampleTasks(): Task[] {
  return [
    {
      id: 1,
      title: "Add retry to webhook sender",
      body: "Webhooks drop on 502.",
      acceptance: "Retries 5 times with backoff",
      status: "in-progress",
      priority: 1,
      jira: "BILL-881",
      repo: "billing-svc",
      tags: ["backend"],
      due: day(1),
      branch: "tars/1-webhook-retry",
      worktree: "/work/billing-1",
      created_at: iso(600),
      updated_at: iso(5),
      unread: true,
      needs_you: true,
      report: {
        id: 1,
        task_id: 1,
        at: iso(5),
        session_id: "sess-42",
        done: "Backoff, 5 tries",
        verified: "make test: 212 passed",
        needs_you: "Alert #oncall or #billing-alerts?",
        next: "Open PR",
      },
      sessions: [
        {
          id: "sess-42",
          name: "webhook retry",
          launch_dir: "/work/billing",
          first_seen: iso(60),
          last_seen: iso(5),
          first_at: iso(60),
          last_at: iso(5),
        },
      ],
      log: [
        { id: 1, at: iso(60), author: "user", kind: "note", text: "Started looking" },
        { id: 2, at: iso(5), author: "agent", kind: "report", text: "Reported: Backoff, 5 tries" },
      ],
    },
    {
      id: 2,
      title: "Rotate staging keys",
      status: "blocked",
      priority: 0,
      waiting_on: "Rahul",
      created_at: iso(900),
      updated_at: iso(300),
    },
    {
      id: 3,
      title: "Tidy README",
      status: "ready",
      priority: 3,
      created_at: iso(900),
      updated_at: iso(30),
      unread: true,
    },
    {
      id: 4,
      title: "Ship CSV export",
      pr: "https://github.com/acme/billing/pull/412",
      status: "in-review",
      priority: 2,
      due: day(-2),
      created_at: iso(2000),
      updated_at: iso(100),
    },
    {
      id: 5,
      title: "Old cleanup",
      status: "done",
      priority: 2,
      created_at: iso(5000),
      updated_at: iso(1000),
      done_at: iso(1000),
    },
  ];
}

// The sample tasks, with every unfinished one moved to in progress.
export function openTasks(): Task[] {
  return sampleTasks().map((t) => (t.status === "done" ? t : { ...t, status: "in-progress" }));
}

export function sampleUnfiled(): Capture[] {
  return [
    {
      session_id: "aaaa1111-2222",
      state: "unfiled",
      repo: "billing-svc",
      root: "/Users/me/code/billing-svc",
      worktree: "/Users/me/code/billing-svc",
      branch: "speed-up-exports",
      prompt: "Exports time out on big accounts\nstream rows instead",
      edits: 6,
      commits: 1,
      files: ["export/csv.go", "export/csv_test.go"],
      started_at: iso(90),
      ended_at: iso(20),
      name: "Stream CSV exports",
    },
  ];
}

const usageLine = (key: string, cost: number, extra: Partial<UsageLine> = {}): UsageLine => ({
  key,
  cost_usd: cost,
  cost: { input: cost * 0.1, output: cost * 0.5, cache_read: cost * 0.1, cache_write: cost * 0.3 },
  tokens: { input: 1000, output: 5000, cache_read: 900_000, cache_write: 40_000 },
  total_tokens: 946_000,
  cache_savings_usd: cost * 0.8,
  calls: cost > 0 ? 10 : 0,
  sessions: cost > 0 ? 2 : 0,
  ...extra,
});

/** Usage over `days` days (0 for all time), shaped like `tars usage --json`. */
export function sampleUsage(days = 30): UsageSummary {
  const n = days || 45;
  const daily = Array.from({ length: n }, (_, i) => usageLine(day(i - n + 1), i % 3 === 0 ? 0 : 2 + (i % 5)));
  const total = daily.reduce((s, d) => s + d.cost_usd, 0);
  return {
    since: daily[0].key,
    until: daily[n - 1].key,
    time_zone: "Asia/Kolkata",
    generated_at: NOW.toISOString(),
    total: usageLine("total", total, { sessions: 12, calls: 420 }),
    daily,
    models: [usageLine("claude-opus-5-5", total * 0.8), usageLine("claude-haiku-5-5", total * 0.2), usageLine("mystery-model", 0, { calls: 3, unpriced_calls: 3 })],
    repos: [usageLine("billing-svc", total * 0.7), usageLine("promos-ui", total * 0.3)],
    tasks: [usageLine("1", total * 0.4, { label: "Add retry to webhook sender", status: "in-progress" })],
    unpriced_models: ["mystery-model"],
    pricing: { source: "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json", fetched_at: NOW.toISOString(), status: "cached", models: 2400 },
    fx: { base: "USD", rates: { INR: 94.91 }, date: day(-1), source: "European Central Bank (frankfurter.dev)", fetched_at: NOW.toISOString(), status: "fresh" },
    files: 396,
    scan_ms: 1840,
  };
}

export function sampleTodos(): TodoList[] {
  const at = iso(30);
  const item = (id: number, list_id: number, position: number, text: string, done = false): TodoItem => ({
    id,
    list_id,
    position,
    text,
    done,
    created_at: at,
    updated_at: at,
    ...(done ? { done_at: at } : {}),
  });
  return [
    {
      id: 1,
      name: "Release",
      position: 0,
      created_at: at,
      updated_at: at,
      items: [item(1, 1, 0, "Update CHANGELOG"), item(2, 1, 1, "Tag v0.4.0", true), item(3, 1, 2, "Publish the cask")],
    },
    { id: 2, name: "Cleanup", position: 1, created_at: at, updated_at: at, items: [item(4, 2, 0, "Drop the legacy sync flag")] },
  ];
}

export interface Backend {
  unfiled: Capture[];
  capture: { on: boolean; off_repos: string[] };
  tasks: Task[];
  repos: Repo[];
  sections: Record<string, Section[]>;
  profile: Section[];
  folder: string | null;
  calls: [string, Record<string, unknown> | undefined][];
  fail: Partial<Record<string, CliError>>;
  callsTo: (cmd: string) => (Record<string, unknown> | undefined)[];
  todos: TodoList[];
  /** While set, todo writes wait for it, so tests can see the optimistic UI. */
  hold: Promise<void> | null;
  /** Holds todo writes until the returned function is called. */
  holdTodos: () => () => void;
  /** What usage_summary returns, given its days and task. */
  usage: (days: number, task: number | null) => UsageSummary;
}

export function fakeBackend(initial: Task[] = sampleTasks()): Backend {
  const b: Backend = {
    tasks: structuredClone(initial),
    unfiled: sampleUnfiled(),
    capture: { on: true, off_repos: [] },
    repos: [{ name: "billing-svc", path: "/Users/me/code/billing-svc", updated_at: NOW.toISOString() }],
    folder: "/Users/me/code/promos-ui",
    sections: {
      "billing-svc": [
        { name: "dod", content: "Tests pass and the PR links the Jira key." },
        { name: "tests", content: "Run `make test`.\n- Integration tests need docker compose up db\n- Skip flaky_* locally" },
      ],
    },
    profile: [{ name: "commits", content: "One logical change per commit." }],
    calls: [],
    fail: {},
    callsTo: (cmd) => b.calls.filter(([c]) => c === cmd).map(([, a]) => a),
    todos: sampleTodos(),
    usage: (days) => sampleUsage(days),
    hold: null,
    holdTodos: () => {
      let release = () => {};
      b.hold = new Promise((r) => (release = r));
      return () => {
        b.hold = null;
        release();
      };
    },
  };
  const todoFail = (message: string) => ({ kind: "failed", message });
  const list = (id: unknown) => {
    const l = b.todos.find((x) => x.id === id);
    if (!l) throw todoFail(`todo list ${id}: not found`);
    return l;
  };
  const item = (id: unknown) => {
    for (const l of b.todos) {
      const it = l.items.find((x) => x.id === id);
      if (it) return { l, it };
    }
    throw todoFail(`todo ${id}: not found`);
  };
  const nextId = (xs: { id: number }[]) => Math.max(0, ...xs.map((x) => x.id)) + 1;
  const renumberTodos = () =>
    b.todos.forEach((l, i) => {
      l.position = i;
      l.items.forEach((it, j) => ((it.position = j), (it.list_id = l.id)));
    });
  const find = (id: unknown) => {
    const t = b.tasks.find((x) => x.id === id);
    if (!t) throw { kind: "failed", message: `task ${id}: not found` };
    return t;
  };
  vi.mocked(invoke).mockImplementation(async (cmd: string, raw?: unknown) => {
    const args = raw as Record<string, unknown> | undefined;
    b.calls.push([cmd, args]);
    if (cmd.startsWith("todo_") && b.hold) await b.hold;
    const err = b.fail[cmd];
    if (err) throw err;
    switch (cmd) {
      case "list_todos":
        return structuredClone(b.todos);
      case "todo_new_list": {
        const name = (args!.name as string).trim();
        if (b.todos.some((l) => l.name.toLowerCase() === name.toLowerCase()))
          throw todoFail(`there's already a todo list called "${name}"`);
        const at = new Date().toISOString();
        const l: TodoList = { id: nextId(b.todos), name, position: b.todos.length, created_at: at, updated_at: at, items: [] };
        b.todos.push(l);
        return structuredClone(l);
      }
      case "todo_rename_list": {
        const l = list(args!.id);
        l.name = (args!.name as string).trim();
        return structuredClone(l);
      }
      case "todo_move_list": {
        const l = list(args!.id);
        const to = (args!.position as number) - 1;
        if (to < 0 || to >= b.todos.length) throw todoFail(`position ${to + 1} is out of range`);
        b.todos = b.todos.filter((x) => x !== l);
        b.todos.splice(to, 0, l);
        renumberTodos();
        return structuredClone(l);
      }
      case "todo_remove_list": {
        const l = list(args!.id);
        if (l.items.length && !args!.force) throw todoFail(`${l.name} still has todos; add --force`);
        b.todos = b.todos.filter((x) => x !== l);
        renumberTodos();
        return { deleted: l.id, name: l.name };
      }
      case "todo_add": {
        const l = list(args!.list);
        const at = new Date().toISOString();
        const it: TodoItem = {
          id: nextId(b.todos.flatMap((x) => x.items)),
          list_id: l.id,
          text: (args!.text as string).trim(),
          done: false,
          position: l.items.length,
          created_at: at,
          updated_at: at,
        };
        l.items.push(it);
        return structuredClone(it);
      }
      case "todo_edit": {
        const { it } = item(args!.id);
        it.text = (args!.text as string).trim();
        return structuredClone(it);
      }
      case "todo_check": {
        const { it } = item(args!.id);
        it.done = args!.done as boolean;
        return [structuredClone(it)];
      }
      case "todo_move": {
        const { l, it } = item(args!.id);
        const into = args!.list === null ? l : list(args!.list);
        const to = (args!.position as number) - 1;
        l.items = l.items.filter((x) => x !== it);
        if (to < 0 || to > into.items.length) {
          l.items.splice(it.position, 0, it);
          throw todoFail(`position ${to + 1} is out of range`);
        }
        into.items.splice(to, 0, it);
        renumberTodos();
        return structuredClone(it);
      }
      case "todo_remove": {
        const { l, it } = item(args!.id);
        l.items = l.items.filter((x) => x !== it);
        renumberTodos();
        return { deleted: [it.id] };
      }
      case "todo_clear": {
        const l = list(args!.list);
        const n = l.items.filter((x) => x.done).length;
        l.items = l.items.filter((x) => !x.done);
        renumberTodos();
        return { list: l.id, cleared: n };
      }
      case "cli_status":
        return { bin: "/Users/me/go/bin/tars", version: "dev" };
      case "list_tasks":
        return b.tasks.map(({ report: _r, sessions: _s, log: _l, ...t }) => structuredClone(t));
      case "show_task":
        return structuredClone(find(args!.id));
      case "set_status": {
        const t = find(args!.id);
        t.status = args!.status as Task["status"];
        t.done_at = t.status === "done" ? new Date().toISOString() : undefined;
        return [structuredClone(t)];
      }
      case "answer_task": {
        const t = find(args!.id);
        if (!t.report?.needs_you) throw { kind: "failed", message: `#${t.id} has no open question` };
        t.report.answer = args!.text as string;
        t.report.answered_at = new Date().toISOString();
        t.needs_you = false;
        return { id: t.id, answered: true };
      }
      case "add_note": {
        const t = find(args!.id);
        t.log = [...(t.log ?? []), { id: 99, at: new Date().toISOString(), author: "user", kind: "note", text: args!.text as string }];
        return { id: t.id, logged: true };
      }
      case "mark_seen": {
        const t = find(args!.id);
        t.unread = false;
        return { seen: [t.id] };
      }
      case "add_task": {
        const input = args!.task as { title: string; priority?: string };
        const t: Task = {
          id: Math.max(0, ...b.tasks.map((x) => x.id)) + 1,
          title: input.title,
          status: "inbox",
          priority: ["urgent", "high", "normal", "low"].indexOf(input.priority ?? "normal"),
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        b.tasks.push(t);
        return structuredClone(t);
      }
      case "edit_task": {
        const t = find(args!.id);
        const e = args!.edit as Record<string, unknown> & { tag?: string[]; untag?: string[] };
        for (const k of ["title", "body", "jira", "repo", "pr"] as const) {
          if (typeof e[k] === "string") (t as unknown as Record<string, unknown>)[k] = e[k] || undefined;
        }
        if (typeof e.due === "string") t.due = e.due === "none" ? undefined : e.due;
        if (typeof e.priority === "string") t.priority = ["urgent", "high", "normal", "low"].indexOf(e.priority);
        const tags = (t.tags ?? []).filter((x) => !e.untag?.includes(x));
        for (const x of e.tag ?? []) if (!tags.includes(x)) tags.push(x);
        t.tags = tags.length ? tags : undefined;
        return structuredClone(t);
      }
      case "delete_task":
        find(args!.id);
        b.tasks = b.tasks.filter((t) => t.id !== args!.id);
        return { deleted: [args!.id] };
      case "list_repos":
        return structuredClone(b.repos);
      case "set_repo": {
        const name = args!.name as string;
        const path = (args!.path as string).replace(/^~/, "/Users/me");
        if (!path.startsWith("/")) throw { kind: "failed", message: "Use the folder's full path, like ~/code/billing-svc." };
        const r = { name, path, updated_at: new Date().toISOString() };
        b.repos = [...b.repos.filter((x) => x.name !== name), r].sort((x, y) => x.name.localeCompare(y.name));
        return r;
      }
      case "show_repo": {
        const r = b.repos.find((x) => x.name === args!.name);
        if (!r) throw { kind: "failed", message: `repo profile "${args!.name}" not found` };
        return { ...structuredClone(r), sections: structuredClone(b.sections[r.name] ?? []) };
      }
      case "get_profile":
        return structuredClone(b.profile);
      case "rename_repo": {
        const from = args!.from as string;
        const to = (args!.to as string).trim();
        if (b.repos.some((r) => r.name === to)) throw { kind: "failed", message: `there's already a repo called "${to}"` };
        const r = b.repos.find((x) => x.name === from)!;
        r.name = to;
        b.sections[to] = b.sections[from] ?? [];
        delete b.sections[from];
        for (const t of b.tasks) if (t.repo === from) t.repo = to;
        return structuredClone(r);
      }
      case "set_repo_section":
      case "set_profile_section": {
        const list =
          cmd === "set_profile_section" ? b.profile : (b.sections[args!.name as string] ??= []);
        const name = (args!.section as string).trim().toLowerCase();
        const text = args!.text as string;
        const i = list.findIndex((x) => x.name === name);
        if (!text.trim()) {
          if (i >= 0) list.splice(i, 1);
        } else if (i >= 0) list[i].content = text;
        else list.push({ name, content: text });
        return { saved: true };
      }
      case "remove_repo":
        b.repos = b.repos.filter((x) => x.name !== args!.name);
        return { deleted: args!.name };
      case "list_unfiled":
        return structuredClone(b.unfiled);
      case "list_captured":
        return structuredClone([
          ...b.unfiled,
          ...b.tasks.flatMap((t) =>
            (t.sessions ?? []).map((x): Capture => ({
              session_id: x.id,
              state: "linked",
              task_id: t.id,
              name: x.name,
              edits: 1,
              commits: 0,
              started_at: x.first_seen,
              ended_at: x.last_seen,
            })),
          ),
        ]);
      case "file_session": {
        const c = b.unfiled.find((x) => x.session_id === args!.session);
        if (!c) throw { kind: "failed", message: `session ${args!.session}: not found` };
        let id = args!.into as number | null;
        if (id === null) {
          id = Math.max(0, ...b.tasks.map((x) => x.id)) + 1;
          b.tasks.push({
            id,
            title: (args!.title as string) || c.name || "",
            repo: c.repo,
            status: "inbox",
            priority: 2,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          });
        } else find(id);
        b.unfiled = b.unfiled.filter((x) => x !== c);
        return { task_id: id, session_id: c.session_id };
      }
      case "unlink_session": {
        const t = find(args!.task);
        const s = t.sessions?.find((x) => x.id === args!.session);
        if (!s) throw { kind: "failed", message: `session ${args!.session} isn't on #${t.id}` };
        t.sessions = t.sessions!.filter((x) => x !== s);
        b.unfiled.push({ session_id: s.id, state: "unfiled", name: s.name, edits: 1, commits: 0, started_at: s.first_seen });
        return { task_id: t.id, session_id: s.id };
      }
      case "dismiss_session":
        b.unfiled = b.unfiled.filter((x) => x.session_id !== args!.session);
        return { dismissed: [args!.session] };
      case "merge_tasks": {
        const from = find(args!.from);
        const into = find(args!.into);
        into.sessions = [...(into.sessions ?? []), ...(from.sessions ?? [])];
        b.tasks = b.tasks.filter((t) => t !== from);
        return { from: from.id, into: into.id };
      }
      case "capture_settings": {
        const on = args!.on as boolean | null;
        const repo = args!.repo as string | null;
        if (on !== null) {
          if (repo) {
            const r = b.repos.find((x) => x.name === repo)!;
            r.capture_off = !on;
            b.capture.off_repos = b.repos.filter((x) => x.capture_off).map((x) => x.name);
          } else b.capture.on = on;
        }
        return structuredClone(b.capture);
      }
      case "usage_summary":
        return b.usage(args!.days as number, (args!.task as number | null) ?? null);
      case "choose_folder":
        return b.folder;
      case "set_zoom":
      case "open_in_terminal":
        return "Opened in a new Ghostty tab";
      case "copy_text":
      case "reveal_path":
      case "open_url":
        return null;
    }
    throw new Error(`unexpected command ${cmd}`);
  });
  return b;
}
