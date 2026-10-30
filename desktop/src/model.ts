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

/** Tasks in a view, grouped by priority, in display order. */
export function groupTasks(tasks: Task[], view: number): Group[] {
  const inView = tasks.filter(VIEWS[view].has);
  inView.sort((a, b) => {
    if (a.priority !== b.priority) return a.priority - b.priority;
    if (!a.due !== !b.due) return a.due ? -1 : 1;
    if ((a.due ?? "") !== (b.due ?? "")) return (a.due ?? "") < (b.due ?? "") ? -1 : 1;
    if (view === DONE) return (b.done_at ?? "").localeCompare(a.done_at ?? "");
    return a.id - b.id;
  });
  const groups: Group[] = [];
  for (const t of inView) {
    const label = PRIORITIES[clampPriority(t.priority)];
    const last = groups[groups.length - 1];
    if (last?.label === label) last.tasks.push(t);
    else groups.push({ label, tasks: [t] });
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
