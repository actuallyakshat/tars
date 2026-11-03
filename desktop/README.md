# Tars for macOS

A desktop window onto the same task list as `tars` in the terminal: views on the left (All, Needs you, Active, Done, Unfiled), a one-line-per-task list in the middle, and the task with its latest agent report, linked Claude sessions and activity on the right. It refreshes every few seconds, so agent work shows up as it happens.

The app has no database code of its own. Every read and write runs the `tars` CLI with `--json`, so it needs a build of tars that has `tars report` (newer than 0.1.3). The app looks for the binary in `$TARS_BIN`, then on `PATH`, then in `/opt/homebrew/bin`, `/usr/local/bin`, `~/go/bin` and `~/.local/bin`, and uses the first one that's new enough. `TARS_DB` is passed as `--db`.

Which binary it looks for depends on the build. Release builds (the Tars app) use `tars` and your real tasks. Debug builds (`tauri dev`) and Tars Dev use `tars-dev` and its own database, so working on the app never touches your real tasks. Install `tars-dev` with `tars dev update` (see the main README), copy your real tasks into it with `tars dev seed`, and rerun `tars dev update` after Go changes so the app and CLI agree.

| Key | Action |
|---|---|
| `j k` / `↑↓`, `home` `end` | Move |
| `enter` / `esc` | Focus the details / back to the list |
| `1`–`4`, `tab` `⇧tab` | Switch view |
| `/` | Filter the list |
| `g` | Change grouping (priority, due, status, repo, none) |
| `x` | Done (again: reopen) |
| `s` / `p` | Change status / priority |
| `u` | Undo the last status or priority change |
| `e` | Edit the task |
| `⌘⌫` | Delete the task (asks first). Right-click a task for Edit, Done and Delete |
| `a` | Answer the agent's question, or change your answer |
| `n` | Add a note |
| `c` / `⌘N` | New task |
| `o` | Copy the latest session's resume command |
| `l` | Expand / collapse activity |
| `f` | Open the selected task full screen (`f` or `esc` to go back) |
| `⌘K` | Search tasks and commands |
| `⌘B` / `⌘⇧B` | Show or hide the sidebar / preview. Drag either divider to resize (the sidebar stops at 320px); double-click resets |
| `⌘+` `⌘−` `⌘0` | Zoom |
| `⌘R` | Refresh |
| `?` | Legend |

Grouping and sort order are in the list header's display menu. The Repos page (sidebar) shows each repo's preferences and your personal profile, and is where you add repos and set their folders. Settings → General chooses whether tasks open in the side pane or full screen. If the app can't load, it retries on its own every few seconds. Jira keys open in the browser once you set your Jira URL in Settings; until then clicking one copies it. Opening a task clears its ●.

## Development

Needs Node, Rust and Xcode's command line tools, plus Go for the Rust integration tests. The Tauri CLI is a dev dependency.

```bash
cd desktop
npm install
npm run tauri dev                          # run the app against tars-dev and the dev database
TARS_BIN=tars npm run tauri dev            # or against the released tars and your real tasks
npm test                                   # component tests (Vitest)
npm run build                              # type-check and build the UI
cd src-tauri && cargo test                 # CLI wrapper tests; builds the Go CLI from this repo
npm run tauri build -- --debug --bundles app  # unsigned debug app (uses tars-dev) in src-tauri/target/debug/bundle
```
