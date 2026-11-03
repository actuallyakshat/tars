// Every call to the Rust side goes through here. The Rust side runs the tars
// CLI with --json and hands back what it printed.
import { invoke } from "@tauri-apps/api/core";

// Field names follow the JSON tags in internal/store.
export interface Report {
  id: number;
  task_id: number;
  at: string;
  session_id?: string;
  done: string;
  verified?: string;
  needs_you?: string;
  next?: string;
  answer?: string;
  answered_at?: string;
}

export interface TaskSession {
  id: string;
  name?: string;
  launch_dir?: string;
  first_seen: string;
  last_seen: string;
  first_at: string;
  last_at: string;
  /** Why it's on the task: branch, worktree, pr, jira, claude, user or agent. */
  via?: string;
  /** What this session did for the task, as Claude named it. */
  title?: string;
}

export interface Entry {
  id: number;
  at: string;
  author: string;
  kind: string;
  text: string;
}

export type Status = "inbox" | "ready" | "in-progress" | "in-review" | "blocked" | "done";

export interface Task {
  id: number;
  title: string;
  body?: string;
  status: Status;
  priority: number;
  jira?: string;
  repo?: string;
  tags?: string[];
  due?: string;
  acceptance?: string;
  branch?: string;
  worktree?: string;
  prs?: string[];
  waiting_on?: string;
  follow_up?: string;
  created_at: string;
  updated_at: string;
  done_at?: string;
  unread?: boolean;
  needs_you?: boolean;
  report?: Report;
  sessions?: TaskSession[];
  log?: Entry[];
}

export interface NewTask {
  title: string;
  priority?: string;
  jira?: string;
  repo?: string;
  due?: string;
  tags?: string;
  body?: string;
}

/** Fields to change on a task. Absent fields stay; "" clears, due takes "none". */
export interface TaskEdit {
  title?: string;
  body?: string;
  priority?: string;
  jira?: string;
  repo?: string;
  due?: string;
  tag?: string[];
  untag?: string[];
  pr?: string[];
  unpr?: string[];
}

export interface Repo {
  name: string;
  path?: string;
  updated_at: string;
  /** Claude sessions in this repo aren't captured. */
  capture_off?: boolean;
}

/** A Claude session that changed code but isn't linked to a task yet. */
export interface Capture {
  session_id: string;
  state: string;
  task_id?: number;
  repo?: string;
  root?: string;
  worktree?: string;
  branch?: string;
  /** The session's first prompt. */
  prompt?: string;
  edits: number;
  commits: number;
  files?: string[];
  started_at: string;
  ended_at?: string;
  /** The session's title, from /rename or Claude. */
  name?: string;
}

export interface CaptureSettings {
  on: boolean;
  off_repos: string[];
}

export interface Section {
  name: string;
  content: string;
}

export interface RepoDetail extends Repo {
  sections?: Section[];
}

export interface TodoItem {
  id: number;
  list_id: number;
  text: string;
  done: boolean;
  position: number;
  created_at: string;
  updated_at: string;
  done_at?: string;
}

/** A named checklist, with its items in order. */
export interface TodoList {
  id: number;
  name: string;
  position: number;
  created_at: string;
  updated_at: string;
  items: TodoItem[];
}

/** One row of a usage breakdown (internal/usage). Costs are in USD. */
export interface UsageLine {
  key: string;
  label?: string;
  status?: string;
  cost_usd: number;
  cost: { input: number; output: number; cache_read: number; cache_write: number };
  tokens: { input: number; output: number; cache_read: number; cache_write: number };
  total_tokens: number;
  cache_savings_usd: number;
  calls: number;
  unpriced_calls?: number;
  sessions: number;
}

/** Live data's origin: fresh, cached (within its time to live), stale (fetch failed) or unavailable. */
export type LiveStatus = "fresh" | "cached" | "stale" | "unavailable";

export interface UsageSummary {
  since: string;
  until: string;
  time_zone: string;
  generated_at: string;
  total: UsageLine;
  daily: UsageLine[];
  models: UsageLine[];
  repos: UsageLine[];
  tasks: UsageLine[];
  unpriced_models: string[];
  pricing: { source: string; fetched_at?: string; status: LiveStatus; models: number; error?: string };
  /** Value of one USD in other currencies. Empty rates mean none could ever be fetched. */
  fx: { base: "USD"; rates: Record<string, number>; date?: string; source?: string; fetched_at?: string; status: LiveStatus; error?: string };
  files: number;
  scan_ms: number;
}

export interface CliStatus {
  bin: string;
  version: string;
}

export type CliErrorKind = "missing" | "too_old" | "failed" | "bad_output" | "system";

export interface CliError {
  kind: CliErrorKind;
  message: string;
}

export function asCliError(e: unknown): CliError {
  if (e && typeof e === "object" && "kind" in e && "message" in e) {
    return e as CliError;
  }
  return { kind: "system", message: e instanceof Error ? e.message : String(e) };
}

/** Missing or outdated CLI: nothing works until the user fixes it. */
export function isFatal(e: CliError) {
  return e.kind === "missing" || e.kind === "too_old";
}

export const api = {
  status: () => invoke<CliStatus>("cli_status"),
  list: () => invoke<Task[]>("list_tasks"),
  show: (id: number) => invoke<Task>("show_task", { id }),
  add: (task: NewTask) => invoke<Task>("add_task", { task }),
  edit: (id: number, edit: TaskEdit) => invoke<Task>("edit_task", { id, edit }),
  remove: (id: number) => invoke("delete_task", { id }),
  setStatus: (id: number, status: Status) => invoke<Task[]>("set_status", { id, status }),
  answer: (id: number, text: string) => invoke("answer_task", { id, text }),
  note: (id: number, text: string) => invoke("add_note", { id, text }),
  seen: (id: number) => invoke("mark_seen", { id }),
  copy: (text: string) => invoke<void>("copy_text", { text }),
  openInTerminal: (sessionId: string, launchDir?: string) =>
    invoke<string>("open_in_terminal", { sessionId, launchDir: launchDir || null }),
  reveal: (path: string) => invoke<void>("reveal_path", { path }),
  openUrl: (url: string) => invoke<void>("open_url", { url }),
  repos: () => invoke<Repo[]>("list_repos"),
  setRepo: (name: string, path: string) => invoke<Repo>("set_repo", { name, path }),
  showRepo: (name: string) => invoke<RepoDetail>("show_repo", { name }),
  profile: () => invoke<Section[]>("get_profile"),
  renameRepo: (from: string, to: string) => invoke<Repo>("rename_repo", { from, to }),
  setRepoSection: (name: string, section: string, text: string) => invoke("set_repo_section", { name, section, text }),
  setProfileSection: (section: string, text: string) => invoke("set_profile_section", { section, text }),
  removeRepo: (name: string) => invoke("remove_repo", { name }),
  chooseFolder: () => invoke<string | null>("choose_folder"),
  zoom: (scale: number) => invoke<void>("set_zoom", { scale }),
  unfiled: () => invoke<Capture[]>("list_unfiled"),
  /** Sessions captured in the last week, linked to a task or unfiled. */
  captured: () => invoke<Capture[]>("list_captured"),
  /** Files a session into task `into`, or a new task (titled `title`) when into is null. */
  fileSession: (session: string, into: number | null, title = "") =>
    invoke<{ task_id: number }>("file_session", { session, into, title }),
  /** Takes a session off a task; one that changed code goes back to Unfiled. */
  unlink: (task: number, session: string) => invoke("unlink_session", { task, session }),
  dismiss: (session: string) => invoke("dismiss_session", { session }),
  merge: (from: number, into: number) => invoke("merge_tasks", { from, into }),
  /** Usage over the last `days` days (0 for everything), or one task's over all time. */
  usage: (days: number, task: number | null = null) => invoke<UsageSummary>("usage_summary", { days, task }),
  todos: () => invoke<TodoList[]>("list_todos"),
  todoNewList: (name: string) => invoke<TodoList>("todo_new_list", { name }),
  todoRenameList: (id: number, name: string) => invoke<TodoList>("todo_rename_list", { id, name }),
  /** Positions count from 1. */
  todoMoveList: (id: number, position: number) => invoke<TodoList>("todo_move_list", { id, position }),
  todoRemoveList: (id: number, force: boolean) => invoke("todo_remove_list", { id, force }),
  todoAdd: (list: number, text: string) => invoke<TodoItem>("todo_add", { list, text }),
  todoEdit: (id: number, text: string) => invoke<TodoItem>("todo_edit", { id, text }),
  todoCheck: (id: number, done: boolean) => invoke<TodoItem[]>("todo_check", { id, done }),
  /** Positions count from 1; list moves the item into another list. */
  todoMove: (id: number, position: number, list: number | null = null) => invoke<TodoItem>("todo_move", { id, position, list }),
  todoRemove: (id: number) => invoke("todo_remove", { id }),
  todoClear: (list: number) => invoke<{ cleared: number }>("todo_clear", { list }),
  capture: (on: boolean | null = null, repo: string | null = null) => invoke<CaptureSettings>("capture_settings", { on, repo }),
};
