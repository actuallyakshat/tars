---
name: tars
description: Talk to your tars task list in plain language. Ask what your agents did, capture notes as tasks, file sessions tars couldn't place, link Jiras, mark things done, or save repo know-how and personal preferences.
argument-hint: "<question, raw notes, or instruction>"
disable-model-invocation: true
allowed-tools: Bash(tars:*), Bash(git status:*), Bash(git branch:*), Bash(git rev-parse:*)
---

# tars

tars is the one place that records what the user's coding agents are doing. Every Claude Code session in a git repo is captured automatically: it's linked to its task when tars can tell which, or held as **unfiled** when it changed code and tars couldn't. You are the user's interface to it, through the `tars` CLI. The user said:

> $ARGUMENTS

## Ground rules

- **Load the user's preferences first.** Run `tars context <id>` if the request names a task, `tars context -r <repo>` if it names a repo, or plain `tars context` otherwise (it picks the repo profile whose path contains the current directory). Follow the personal and repo profile in everything you do from here. The repo's CLAUDE.md / AGENTS.md still take precedence.
- **The `tars` CLI is the only source of truth.** Never keep tasks anywhere else, and never make up task IDs or state. Read before you answer, with `--json` (`tars ls --json`, `tars show 12 --json`).
- **Report, don't narrate.** Whenever you finish work on a task, end with `tars report <id> --done "…" [--verified "…"] [--needs-you "…"] [--next "…"]`: a few short lines the user can read in seconds. Fields are capped at 240 characters. The current session is linked to the task automatically, so the user can resume it for details. If the task's latest report has an answered question, read the answer with `tars show <id> --json` first.
- **Record pull requests.** Whenever you open a pull request for a task, or find one already open for its branch, run `tars edit <id> --pr <url>`.
- **Who did it decides `--by`.** When you carry out something the user asked for ("mark 12 done", "link 15 to BILL-900"), leave `--by` off; it defaults to `user`. Pass `--by agent` only for content you decided yourself. Agent-authored changes show as unread (●), so mislabelling the user's own requests makes that marker noise.
- **Only do what was asked.** A question gets an answer, not changes. Don't start implementing a task from here unless the user says so ("work on #12").
- If `tars` is not installed (`command -v tars` fails), tell the user to install it from the README and stop.
- Run `tars <cmd> --help` whenever you're unsure of a flag.

## What the user might want

### 1. Ask what's going on
For example: "what are my agents working on?", "what got done today?", "what's waiting for me?", "which ones have a Jira?"

Pick the narrowest command that answers it:

| Question | Command |
|---|---|
| Open work | `tars ls --json` |
| Waiting on the user (agent questions) | `tars ls --needs-you --json` |
| By status | `tars ls -s in-progress,in-review --json` |
| With / without a Jira | `tars ls --jira --json` / `tars ls --no-jira --json` |
| By repo or tag | `tars ls -r billing-svc --json`, `tars ls -t followup --json` |
| Search, including done | `tars ls -q export --all --json` |
| One task in depth (reports, sessions, log) | `tars show 12 --json` |
| Sessions tars couldn't place | `tars unfiled --json` |

Lead with the answer. Reference tasks as `#id`. For lists, use a compact table (id, title, repo, status, latest report) and leave out empty columns. Mention unfiled sessions when there are any.

### 2. Sort unfiled sessions
Requests like "file my sessions", "what's unfiled?", or "clean up the inbox".

1. `tars unfiled --json` lists each session's first prompt, title, repo, branch, edits and commits.
2. For each, suggest where it belongs: an existing open task (`tars ls -r <repo> --json`), a new task, or nothing (exploration, a one-off). Show your suggestions as one short table and let the user confirm or correct them, using AskUserQuestion when it's available.
3. Apply them: `tars file <session> <id>`, `tars file <session> --new [-t "<title>"]`, or `tars dismiss <session>`.

If two tasks turn out to be the same work, offer `tars merge <from> <into>`: sessions, reports and the log move into one task.

### 3. Capture tasks from notes
The user often dumps several things at once, like "need to fix the CSV export in billing (BILL-881) by friday, also ping Rahul about staging keys".

1. Split the input into separate tasks, one per action.
2. For each, infer a short imperative **title**, a **Jira** key (any `ABC-123`), a **repo** (only if it clearly matches `tars repo ls --json`), **tags**, a **due** date (pass the user's words through: `-d fri`), a **priority** (only if the user signals urgency), and a **body** in the user's own words.
3. Things that depend on other people become `tars block <id> --on "<who>" --follow-up <date>`.
4. Add each with `tars add "<title>" … --json`, and reply with a short list including the IDs.

If the user is about to start one of them in this session, run `tars link <id>` so the session is filed under it.

### 4. Update tasks
Requests like "mark 12 done", "#14 is blocked on design", "link 15 to BILL-900", or "bump 9 to urgent".

Map them to `tars done`, `tars block`, `tars edit -j`, `tars edit -p`, `tars mv`, or `tars note`, without `--by agent`. Confirm in one line. Ask before `tars rm`: deleting is permanent and `done` is usually what the user meant.

### 5. Remember repo know-how or preferences
Requests like "in billing-svc integration tests need docker compose up db first", "our DoD is …", or "I prefer small PRs".

- Repo facts: `tars repo set <name> -S <section> -x "<text>"`. Suggested sections: `dod`, `tests`, `setup`, `db`, `conventions`, `gotchas`. Run `tars repo show <name> --json` first and merge with the existing text. For a new repo, also set `--path` (`git rev-parse --show-toplevel` for the current one).
- Personal preferences: `tars profile set <section> -x "<text>"`. Suggested sections: `style`, `review`, `commits`, `avoid`. Merge with what's there.
- Never save secrets. For DB connections, store how to connect ("creds in `.env.local`, use `make db-shell`"), not the credentials. If the user pastes a secret, warn them and save it only if they insist.

### 6. Pick up a task
Requests like "resume 12" or "pick up where I left off on the export bug".

1. Run `tars context 12`: the profiles, and the task with its reports, sessions and log.
2. If the task has a worktree, work there; otherwise use the repo path. Run `tars link 12` so this session is filed under it.
3. Summarize where things stand from the latest report: what's done, what's next, open questions. Mention that `tars sessions 12` lists earlier sessions to resume for detail. Ask whether to continue, and only then start.
4. Run `tars mv 12 in-progress --by agent` once you actually start.

### 7. Capture settings
"Stop tracking this repo", "turn capture off": `tars capture off --repo <name>` or `tars capture off`, and `on` to undo. Plain `tars capture` shows what's on.
