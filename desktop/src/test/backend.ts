// An in-memory stand-in for the Rust commands, so components run against
// something that behaves like the CLI.
import { invoke } from "@tauri-apps/api/core";
import { vi } from "vitest";
import type { CliError, Repo, Task } from "../api";

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

export interface Backend {
  tasks: Task[];
  repos: Repo[];
  folder: string | null;
  calls: [string, Record<string, unknown> | undefined][];
  fail: Partial<Record<string, CliError>>;
  callsTo: (cmd: string) => (Record<string, unknown> | undefined)[];
}

export function fakeBackend(initial: Task[] = sampleTasks()): Backend {
  const b: Backend = {
    tasks: structuredClone(initial),
    repos: [{ name: "billing-svc", path: "/Users/me/code/billing-svc", updated_at: NOW.toISOString() }],
    folder: "/Users/me/code/promos-ui",
    calls: [],
    fail: {},
    callsTo: (cmd) => b.calls.filter(([c]) => c === cmd).map(([, a]) => a),
  };
  const find = (id: unknown) => {
    const t = b.tasks.find((x) => x.id === id);
    if (!t) throw { kind: "failed", message: `task ${id}: not found` };
    return t;
  };
  vi.mocked(invoke).mockImplementation(async (cmd: string, raw?: unknown) => {
    const args = raw as Record<string, unknown> | undefined;
    b.calls.push([cmd, args]);
    const err = b.fail[cmd];
    if (err) throw err;
    switch (cmd) {
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
        for (const k of ["title", "body", "jira", "repo"] as const) {
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
      case "remove_repo":
        b.repos = b.repos.filter((x) => x.name !== args!.name);
        return { deleted: args!.name };
      case "choose_folder":
        return b.folder;
      case "set_zoom":
      case "copy_text":
      case "open_in_terminal":
      case "reveal_path":
      case "open_url":
        return null;
    }
    throw new Error(`unexpected command ${cmd}`);
  });
  return b;
}
