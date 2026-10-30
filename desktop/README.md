# Tars for macOS

A desktop window onto the same task list as `tars` in the terminal: views on the left (Needs you first), a one-line-per-task list in the middle, and the task with its latest agent report, linked Claude sessions and activity on the right. It refreshes every few seconds, so agent work shows up as it happens.

The app has no database code of its own. Every read and write runs the `tars` CLI with `--json`, so it needs a build of tars that has `tars report` (newer than 0.1.3). The app looks for the binary in `$TARS_BIN`, then on `PATH`, then in `/opt/homebrew/bin`, `/usr/local/bin` and `~/go/bin`, and uses the first one that's new enough. `TARS_HOME` is passed through, and `TARS_DB` is passed as `--db`.

| Key | Action |
|---|---|
| `j k` / `↑↓`, `home` `end` | Move |
| `enter` / `esc` | Focus the details / back to the list |
| `1`–`7`, `tab` `⇧tab` | Switch view |
| `/` | Filter the list |
| `g` | Change grouping (priority, due, status, repo, none) |
| `x` | Done (again: reopen) |
| `s` / `p` | Change status / priority |
| `u` | Undo the last status or priority change |
| `e` | Edit the task |
| `⌘⌫` | Delete the task (asks first). Right-click a task for Edit, Done and Delete |
| `a` | Answer the agent's question |
| `n` | Add a note |
| `c` / `⌘N` | New task |
| `o` | Copy the latest session's resume command |
| `l` | Expand / collapse activity |
| `⌘K` | Search tasks and commands |
| `⌘B` / `⌘⇧B` | Show or hide the sidebar / preview |
| `⌘+` `⌘−` `⌘0` | Zoom |
| `⌘R` | Refresh |
| `?` | Legend |

Grouping and sort order are in the list header's display menu. Repo folders are set in Settings → Repos. Jira keys open in the browser once you set your Jira URL in Settings; until then clicking one copies it. Opening a task clears its ●.

## Development

Needs Node, Rust and Xcode's command line tools, plus Go for the Rust integration tests. The Tauri CLI is a dev dependency.

```bash
cd desktop
npm install
npm run tauri dev                          # run the app against your real tars data
TARS_HOME=/tmp/tars-demo npm run tauri dev # or against a scratch database
npm test                                   # component tests (Vitest)
npm run build                              # type-check and build the UI
cd src-tauri && cargo test                 # CLI wrapper tests; builds the Go CLI from this repo
npm run tauri build -- --debug --bundles app  # unsigned tars.app in src-tauri/target/debug/bundle
```
