# tars

A task ledger that you and your coding agent share.

- **You** get a fast CLI and a compact, keyboard-driven task list in the terminal (`tars`).
- **Your agent** gets a JSON interface and a Claude Code plugin. Talk to it in plain language and it keeps the ledger: it captures raw notes, links Jiras, marks things done, and sweeps ready tasks into tested branches overnight.

The agent only touches the ledger when you invoke it. The plugin's skills can't be triggered by the model on its own.

```
/tars:tars what's pending? which ones have a jira?
/tars:tars fix the CSV export timeout in billing (BILL-881) by friday, also ping Rahul about staging keys
/tars:sweep jira only
/tars:brief
/tars:park
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

**3. The desktop app (macOS, optional).** A window onto the same list; it uses the CLI above.

```bash
brew install --cask actuallyakshat/tap/tars-app
```

**Updating.** Run `/tars:update` in Claude Code: it upgrades the CLI, the plugin and the desktop app, whichever you have, and tells you what to reload. By hand: `brew upgrade --cask tars tars-app` and `/plugin marketplace update tars`.

## The CLI

Run `tars` with no arguments to open the task list. It's a compact, Gmail-style list: one line per task, grouped by priority, with views along the top (Open · Inbox · Ready · Review · Blocked · Done). Press `enter` to open a task. On wide terminals (120+ columns) the details open beside the list; on narrower ones they take the whole screen. The activity log starts collapsed.

A **●** marks tasks the agent has updated since you last opened them.

| Key | Action |
|---|---|
| `↑↓` / `j k` | Move (in full-screen details, `j k` go to the next/previous task) |
| `enter` / `esc` | Open / close details |
| `tab` / `1`–`6` | Switch view |
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
tars brief                     # morning summary
```

Dates accept `2026-11-03`, `today`, `tomorrow`, `fri`, `+3d`, `+2w`, and `none`. Every command takes `--json`.

### Repo profiles and personal profile

These hold the things an agent can't infer from code: your definition of done, how to run tests, how to reach the local DB, and gotchas.

```bash
tars repo set billing-svc --path ~/code/billing-svc
tars repo set billing-svc -S tests -x "make test; integration: docker compose up db && make itest"
tars repo set billing-svc -S dod -f ./docs/DOD.md
tars profile set style -x "Small PRs. No mocks for the DB layer."
tars context 12                # everything an agent should read before working on #12
```

Precedence: the repo's **CLAUDE.md / AGENTS.md** > tars repo profile > personal profile. Don't store secrets. Store *how* to get them (e.g. "creds in .env.local").

## The plugin

| Command | What it does |
|---|---|
| `/tars:tars <anything>` | Ask questions, capture tasks from raw notes, triage the inbox into `ready` (pulls Jira details when the Atlassian tools are available), update tasks, save repo know-how, resume a parked task. |
| `/tars:sweep [filter] [--yes]` | For each `ready` task: create a worktree on `tars/<id>-<slug>`, write tests for the acceptance criteria, implement, run the suite, check the DoD, and **commit locally**. The task moves to `in-review`, or to `blocked` with a question for you. Never pushes. |
| `/tars:brief` | Morning review: what the sweep produced, questions waiting for you, what's due, follow-ups, and stale work. Suggests a focus for the day. |
| `/tars:park [id]` | Saves goal, decisions, dead ends and the next step into the task log so the work can be resumed later. |
| `/tars:update [cli\|plugin\|app]` | Updates the CLI, this plugin and the desktop app to the latest release. Asks before installing anything you don't have, and leaves dev builds alone. |

**The sweep runs unattended once you confirm the plan,** but it still goes through Claude Code's permission system. For an overnight run, set up allow rules for your test, build and git commands, or run the session in a sandboxed permission mode you trust.

### Typical day

1. During the day, dump work as it comes in: `/tars:tars …` or `tars add …` in any shell.
2. In the evening, run `/tars:tars triage my inbox`. It fills in Jira details and acceptance criteria and asks you only for what's missing.
3. At night, run `/tars:sweep jira only --yes`.
4. In the morning, run `/tars:brief`, review the branches, and push the good ones yourself.

## The macOS app

`desktop/` holds a native macOS app (Tauri) for when you'd rather click than type: the same views with Needs you first, the task list, and a detail pane with the agent's report, the linked Claude sessions (copy the resume command or open it in Terminal), and the activity log. You can answer questions, add notes and tasks, and change status from it. It reads and writes only through the `tars` CLI, so it needs a build that has `tars report`. See [desktop/README.md](desktop/README.md) to run or build it.

## Data

Everything lives in one SQLite file at `~/.local/share/tars/tars.db`. Override the location with `$TARS_HOME` or `--db`. Parallel agents can write to it safely (WAL mode).

## Development

```bash
go test ./...
go run ./cmd/tars
go install ./cmd/tars                   # local build into your Go bin directory
scripts/remove-dev-binary.sh            # remove it again, back to the Homebrew tars
goreleaser release --snapshot --clean   # cross-platform binaries in dist/
```

## License

MIT. See [LICENSE](LICENSE).
