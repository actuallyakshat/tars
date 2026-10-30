// Display rules, kept in step with internal/ui (views, grouping, due text).
import type { Status, Task } from "./api";

export interface View {
  name: string;
  has: (t: Task) => boolean;
}

const only = (...statuses: Status[]) => (t: Task) => statuses.includes(t.status);

export const VIEWS: View[] = [
  { name: "Needs you", has: (t) => !!t.needs_you && t.status !== "done" },
  { name: "Open", has: (t) => t.status !== "done" },
  { name: "Inbox", has: only("inbox") },
  { name: "Ready", has: only("ready") },
  { name: "Review", has: only("in-review") },
  { name: "Blocked", has: only("blocked") },
  { name: "Done", has: only("done") },
];

export const NEEDS_YOU = 0;
export const OPEN = 1;
export const DONE = VIEWS.length - 1;

export const STATUSES: Status[] = ["inbox", "ready", "in-progress", "in-review", "blocked", "done"];

export const STATUS: Record<Status, { icon: string; label: string }> = {
  inbox: { icon: "○", label: "Inbox" },
  ready: { icon: "◇", label: "Ready" },
  "in-progress": { icon: "◐", label: "In progress" },
  "in-review": { icon: "◉", label: "In review" },
  blocked: { icon: "■", label: "Blocked" },
  done: { icon: "✔", label: "Done" },
};

export const PRIORITIES = ["Urgent", "High", "Normal", "Low"];

export const clampPriority = (p: number) => Math.min(3, Math.max(0, p | 0));

/** The glyph shown before a task. Normal priority stays blank. */
export function priorityMark(p: number) {
  return ["▲▲", "▲", "", "▽"][clampPriority(p)];
}

export function counts(tasks: Task[]) {
  return VIEWS.map((v) => {
    const inView = tasks.filter(v.has);
    return { total: inView.length, unread: inView.filter((t) => t.unread).length };
  });
}

/** The view to open on: Needs you when it has anything, otherwise Open. */
export function defaultView(tasks: Task[]) {
  return tasks.some(VIEWS[NEEDS_YOU].has) ? NEEDS_YOU : OPEN;
}

export interface Group {
  label: string;
  tasks: Task[];
}

export type GroupBy = "priority" | "due" | "status" | "repo" | "none";
export type SortBy = "priority" | "due" | "updated" | "created" | "title";

export const GROUPINGS: { id: GroupBy; label: string }[] = [
  { id: "priority", label: "Priority" },
  { id: "due", label: "Due date" },
  { id: "status", label: "Status" },
  { id: "repo", label: "Repo" },
  { id: "none", label: "No grouping" },
];

export const SORTS: { id: SortBy; label: string }[] = [
  { id: "priority", label: "Priority, then due" },
  { id: "due", label: "Due date" },
  { id: "updated", label: "Recently updated" },
  { id: "created", label: "Newest first" },
  { id: "title", label: "Title" },
];

// Same order as the TUI: what needs a look first.
const STATUS_ORDER: Record<Status, number> = { "in-review": 0, "in-progress": 1, ready: 2, inbox: 3, blocked: 4, done: 5 };

/** The group a task falls in, and the group's place in the list. */
export function groupOf(t: Task, by: GroupBy, now: Date): [string, number] {
  switch (by) {
    case "priority": {
      const p = clampPriority(t.priority);
      return [PRIORITIES[p], p];
    }
    case "due": {
      const days = daysUntil(t.due, now);
      if (days === null) return ["No date", 5];
      if (days < 0) return ["Overdue", 0];
      if (days === 0) return ["Today", 1];
      if (days === 1) return ["Tomorrow", 2];
      if (days < 7) return ["This week", 3];
      return ["Later", 4];
    }
    case "status":
      return [STATUS[t.status].label, STATUS_ORDER[t.status]];
    case "repo":
      return t.repo ? [t.repo, 0] : ["No repo", 1];
    case "none":
      return ["", 0];
  }
}

/** Every word of the query appears in the task's title, #id, Jira key, repo or tags. */
export function matchesQuery(t: Task, query: string) {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (!words.length) return true;
  const hay = [`#${t.id}`, t.title, t.jira, t.repo, ...(t.tags ?? [])].join(" ").toLowerCase();
  return words.every((w) => hay.includes(w));
}

const byDue = (a: Task, b: Task) => {
  if (!a.due !== !b.due) return a.due ? -1 : 1;
  return (a.due ?? "").localeCompare(b.due ?? "");
};

function compare(a: Task, b: Task, sort: SortBy, view: number) {
  switch (sort) {
    case "due":
      return byDue(a, b) || a.priority - b.priority || a.id - b.id;
    case "updated":
      return b.updated_at.localeCompare(a.updated_at) || b.id - a.id;
    case "created":
      return b.created_at.localeCompare(a.created_at) || b.id - a.id;
    case "title":
      return a.title.localeCompare(b.title, undefined, { sensitivity: "base" }) || a.id - b.id;
    case "priority":
      if (a.priority !== b.priority) return a.priority - b.priority;
      return (
        byDue(a, b) ||
        // Done: most recently finished first
        (view === DONE ? (b.done_at ?? "").localeCompare(a.done_at ?? "") : 0) ||
        a.id - b.id
      );
  }
}

export interface ListOptions {
  group?: GroupBy;
  sort?: SortBy;
  query?: string;
  now?: Date;
}

/** Tasks in a view, filtered, grouped and sorted in display order. */
export function groupTasks(tasks: Task[], view: number, opts: ListOptions = {}): Group[] {
  const { group = "priority", sort = "priority", query = "", now = new Date() } = opts;
  const items = tasks
    .filter((t) => VIEWS[view].has(t) && matchesQuery(t, query))
    .map((t) => ({ t, g: groupOf(t, group, now) }));
  items.sort((a, b) => a.g[1] - b.g[1] || a.g[0].localeCompare(b.g[0]) || compare(a.t, b.t, sort, view));
  const groups: Group[] = [];
  for (const { t, g } of items) {
    const last = groups[groups.length - 1];
    if (last?.label === g[0]) last.tasks.push(t);
    else groups.push({ label: g[0], tasks: [t] });
  }
  return groups;
}

export const flatten = (groups: Group[]) => groups.flatMap((g) => g.tasks);

function startOfDay(d: Date) {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

/** Whole calendar days from now to a YYYY-MM-DD date, or null. */
export function daysUntil(date: string | undefined, now: Date): number | null {
  const m = date?.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!m) return null;
  const day = new Date(+m[1], +m[2] - 1, +m[3]);
  return Math.round((day.getTime() - startOfDay(now).getTime()) / 86_400_000);
}

export type Tone = "late" | "today" | "soon" | "muted";

export function dueText(date: string | undefined, now: Date): { text: string; tone: Tone } {
  if (!date) return { text: "", tone: "muted" };
  const days = daysUntil(date, now);
  if (days === null) return { text: date, tone: "muted" };
  if (days < 0) return { text: `${-days}d late`, tone: "late" };
  if (days === 0) return { text: "today", tone: "today" };
  if (days === 1) return { text: "tomorrow", tone: "soon" };
  const day = new Date(now.getFullYear(), now.getMonth(), now.getDate() + days);
  if (days < 7) return { text: day.toLocaleDateString("en-US", { weekday: "short" }), tone: "muted" };
  return { text: day.toLocaleDateString("en-US", { month: "short", day: "numeric" }), tone: "muted" };
}

export function ago(ts: string | undefined, now: Date): string {
  if (!ts) return "";
  const t = new Date(ts);
  if (isNaN(t.getTime())) return ts;
  const s = (now.getTime() - t.getTime()) / 1000;
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  if (s < 30 * 86400) return `${Math.floor(s / 86400)}d ago`;
  return t.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

/** An agent question still waiting on the user. */
export const openQuestion = (t: Task) =>
  t.report?.needs_you && !t.report.answered_at ? t.report.needs_you : "";

export const resumeCommand = (sessionId: string) => `claude --resume ${sessionId}`;

export function jiraUrl(base: string, key: string) {
  const b = base.trim().replace(/\/+$/, "");
  return b ? `${b}/browse/${encodeURIComponent(key)}` : "";
}
