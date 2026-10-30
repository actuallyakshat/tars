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
  setStatus: (id: number, status: Status) => invoke<Task[]>("set_status", { id, status }),
  answer: (id: number, text: string) => invoke("answer_task", { id, text }),
  note: (id: number, text: string) => invoke("add_note", { id, text }),
  seen: (id: number) => invoke("mark_seen", { id }),
  copy: (text: string) => invoke<void>("copy_text", { text }),
  openInTerminal: (sessionId: string, launchDir?: string) =>
    invoke<void>("open_in_terminal", { sessionId, launchDir: launchDir || null }),
  reveal: (path: string) => invoke<void>("reveal_path", { path }),
  openUrl: (url: string) => invoke<void>("open_url", { url }),
};
