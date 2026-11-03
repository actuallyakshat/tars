# tars

One place for everything your coding agents do.

Install the Claude Code plugin and every Claude session in a git repo is captured on its own:

- **Linked to its task** when tars can tell which one: a `tars/<id>-…` branch, a branch, worktree or PR already on a task, or a Jira key in the branch or prompt. A feature worked on across many sessions is one task with every session on it; once the first one links, the rest on that branch follow.
- **Asked about once** when only the wording resembles an open task: Claude checks with you, then links it.
- **Held as Unfiled** when it changed code and matched nothing. You file it into a task, make it a new task, or dismiss it. Sessions that only read or answer questions leave nothing behind.

Agents leave short reports on their tasks (what's done, how it was verified, what they need from you, what's next), so you read a few lines instead of a transcript, and resume the session when you want the detail.

```
/tars:tars what did my agents do today?
/tars:tars file my unfiled sessions
/tars:tars fix the CSV export timeout in billing (BILL-881) by friday
```

## Install

**1. The CLI.** A single binary. You don't need Go installed.

On macOS, with Homebrew (it asks you to trust a new tap once):

```bash
brew tap actuallyakshat/tap
brew trust --cask actuallyakshat/tap/tars
brew install --cask tars
tars --version
```

On Linux or Windows, download a binary from [GitHub Releases](https://github.com/actuallyakshat/tars/releases/latest). Or build from source with `go install github.com/actuallyakshat/tars/cmd/tars@latest`.

To upgrade later: `brew upgrade --cask tars`.

**2. The Claude Code plugin.** Run these inside Claude Code:

```
/plugin marketplace add actuallyakshat/tars
/plugin install tars@tars
```

Capture starts with your next session. Linking a session or recording a report runs `tars` through Bash, so allow it once (`Bash(tars:*)`) to skip the permission prompts. Turn capture off with `tars capture off`, or for one repo with `tars capture off --repo <name>`.

**3. The desktop app (macOS, optional).** A window onto the same list; it uses the CLI above.

```bash
brew install --cask actuallyakshat/tap/tars-app
```

**Updating.** Run `tars update`. It upgrades the CLI and the desktop app through Homebrew (whichever you have) and the Claude Code plugin, then tells you what to restart. `/tars:update` in Claude Code does the same.

## The CLI

Run `tars` with no arguments to open the task list. It's a compact, Gmail-style list: one line per task, grouped by priority, with views along the top (All · Needs you · Active · Done · Unfiled). Press `enter` to open a task. On wide terminals (120+ columns) the details open beside the list; on narrower ones they take the whole screen. The activity log starts collapsed.

A **●** marks tasks the agent has updated since you last opened them.

| Key | Action |
|---|---|
| `↑↓` / `j k` | Move (in full-screen details, `j k` go to the next/previous task) |
| `enter` / `esc` | Open / close details |
| `tab` / `1`–`5` | Switch view (`5` is Unfiled: `f` file into a task, `n` new task, `d` dismiss) |
| `g` | Group by priority → due → status → repo → none |
| `/` | Search (title, Jira, repo, tags) |
| `x` | Done (press again to reopen) |
| `s` / `p` | Change status / priority |
| `u` | Undo the last change |
| `c` | Quick add: `Fix export BILL-881 !high @billing #perf due:fri` |
| `n` | Add a note to the selected task |
| `l` | Expand / collapse activity |
| `?` | All keys |

```bash
tars add "Fix CSV export timeout" -j BILL-881 -r billing-svc -d fri -p high
tars ls                        # open tasks grouped by status
tars ls --jira -s ready        # filters: -s status, -r repo, -t tag, -q search, --all
tars show 12                   # full card with the agent/user log
tars edit 12 -a "exports 1M rows < 30s" --untag later
tars mv 12 ready               # statuses: inbox ready in-progress in-review blocked done
tars done 12 14
tars block 12 --on "Rahul (staging keys)" --follow-up thu
tars note 12 "root cause: N+1 in exporter"
```

### Captured sessions

```bash
tars unfiled                   # sessions that changed code but matched no task
tars captured                  # sessions captured this week, linked or unfiled
tars file 3f2a9c1e 21          # file one into #21 (a prefix of the session ID is enough)
tars file 3f2a9c1e --new       # make it a new task, titled after the session
tars dismiss 3f2a9c1e          # not task work
tars link 21                   # from inside a Claude session: file this session under #21
tars merge 31 21               # two tasks turned out to be the same work: fold #31 into #21
tars sessions 21               # every session that worked on #21, with resume commands
tars capture                   # what's captured; tars capture off [--repo <name>] to stop
```

Dates accept `2026-11-03`, `today`, `tomorrow`, `fri`, `+3d`, `+2w`, and `none`. Every command takes `--json`.

### Repo profiles and personal profile

These hold the things an agent can't infer from code: your definition of done, how to run tests, how to reach the local DB, and gotchas.

```bash
tars repo set billing-svc --path ~/code/billing-svc
tars repo set billing-svc -S tests -x "make test; integration: docker compose up db && make itest"
tars repo set billing-svc -S dod -f ./docs/DOD.md
tars repo mv billing-svc billing   # rename; its tasks move with it
tars profile set style -x "Small PRs. No mocks for the DB layer."
tars edit 12 --pr https://github.com/acme/billing/pull/412   # repeatable: a task can have several PRs
tars context 12                # everything an agent should read before working on #12
```

Precedence: the repo's **CLAUDE.md / AGENTS.md** > tars repo profile > personal profile. The desktop app's Repos page shows all of it, and lets you rename repos and add, edit or remove preferences. Don't store secrets. Store *how* to get them (e.g. "creds in .env.local").

## The plugin

| Part | What it does |
|---|---|
| Capture hooks | On a session's first prompt, link it to its task (or ask you once when it's a guess). When it ends, hold it as Unfiled if it changed code and isn't linked. Silent when `tars` isn't installed or capture is off. |
| `/tars:tars <anything>` | Ask what your agents did, file unfiled sessions, capture tasks from raw notes, update tasks, save repo know-how and personal preferences, pick up a task. |
| `/tars:update [cli\|plugin\|app]` | Updates the CLI, this plugin and the desktop app to the latest release. Asks before installing anything you don't have, and leaves dev builds alone. |

The plugin never creates, edits or closes tasks on its own. Work that matches no task waits in Unfiled until you sort it.

## The macOS app

`desktop/` holds a native macOS app (Tauri) for when you'd rather click than type: the same views (All, Needs you, Active, Done), Unfiled sessions to file or dismiss, the task list, and a detail pane with the agent's report, the linked Claude sessions (copy the resume command or open it in Terminal), and the activity log. You can answer questions, add notes and tasks, merge tasks, change status, and switch capture on or off from it. It reads and writes only through the `tars` CLI, so it needs a build that has `tars report`. See [desktop/README.md](desktop/README.md) to run or build it.

## Data

Everything lives in one SQLite file at `~/.local/share/tars/tars.db`. Override the location with `$TARS_HOME` or `--db`. Parallel agents and hooks can write to it safely (WAL mode).

## Development

```bash
go test ./...
go run ./cmd/tars
goreleaser release --snapshot --clean   # cross-platform binaries in dist/
```

To release, run `scripts/release.sh 1.2.3 "one-line summary"` on a clean, up-to-date `main`. It bumps the plugin and desktop versions, commits, tags `v1.2.3` and pushes after you confirm. The tag starts CI, which publishes the CLI, Tars.app and both Homebrew casks.

### A dev build next to the release

`tars dev update` builds your checkout and installs it beside the released tars, without touching it:

| | Release | Dev build |
|---|---|---|
| CLI | `tars` (Homebrew) | `tars-dev` in `~/.local/bin` |
| Database | `~/.local/share/tars` | `~/.local/share/tars-dev` (`$TARS_DEV_HOME`) |
| App | Tars | Tars Dev, in /Applications |
| Plugin | `tars`, `/tars:tars` | `tars-dev`, `/tars-dev:tars` |

```bash
go run ./cmd/tars dev update     # the first time, inside the checkout (it remembers the path)
tars dev update                  # later, from anywhere; rebuilds the CLI, plugin and app
tars dev update --seed           # also copy your real tasks into the dev database
tars dev seed [--yes]            # just copy the real tasks in (the real database is only read)
```

Both plugins capture each session: the release one into your real tasks, the dev one into the dev database. Restart Claude Code after `tars dev update` to load the dev plugin. Building the app needs Node and Rust and takes a few minutes. Make sure `~/.local/bin` is on your PATH.

## License

MIT. See [LICENSE](LICENSE).
