# TARS · Track · Assign · Run · Ship

A task ledger that you and your coding agent share.

- **You** get a fast CLI and a full-screen board in the terminal (`tars`).
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

> The Homebrew tap, GitHub Releases and `/plugin marketplace add` work once the repo is published and the first release is cut. Until then, build from source and add the plugin from a local path: `/plugin marketplace add /path/to/tars`.

**CLI.** Download a single static binary. Users don't need Go installed.

```bash
brew install akshat-dubey/tap/tars        # macOS / Linux
# or grab a binary from GitHub Releases, or from source:
go install github.com/akshat-dubey/tars/cmd/tars@latest
```

**Claude Code plugin**

```
/plugin marketplace add akshat-dubey/tars
/plugin install tars@tars
```

## The CLI

Run `tars` with no arguments to open the board. Keys:

| Key | Action |
|---|---|
| `←→↑↓` / `hjkl` | Move between columns and tasks |
| `enter` | Open the task's details and log |
| `[` / `]` | Move the task to the previous or next status |
| `d` | Mark done |
| `+` / `-` | Raise or lower priority |
| `a` | Show or hide done tasks |
| `q` | Quit |

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

**The sweep runs unattended once you confirm the plan,** but it still goes through Claude Code's permission system. For an overnight run, set up allow rules for your test, build and git commands, or run the session in a sandboxed permission mode you trust.

### Typical day

1. During the day, dump work as it comes in: `/tars:tars …` or `tars add …` in any shell.
2. In the evening, run `/tars:tars triage my inbox`. It fills in Jira details and acceptance criteria and asks you only for what's missing.
3. At night, run `/tars:sweep jira only --yes`.
4. In the morning, run `/tars:brief`, review the branches, and push the good ones yourself.

## Data

Everything lives in one SQLite file at `~/.local/share/tars/tars.db`. Override the location with `$TARS_HOME` or `--db`. Parallel agents can write to it safely (WAL mode).

## Development

```bash
go test ./...
go run ./cmd/tars
goreleaser release --snapshot --clean   # cross-platform binaries in dist/
```
