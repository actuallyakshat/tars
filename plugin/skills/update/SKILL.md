---
name: update
description: Update tars itself to the latest release: the tars CLI, the Tars desktop app and this Claude Code plugin. Runs `tars update` and reports what changed.
disable-model-invocation: true
allowed-tools: Bash(tars update), Bash(tars --version), Bash(command -v:*)
---

# /tars:update

1. If `command -v tars` fails, tell the user to install the CLI (`brew install --cask actuallyakshat/tap/tars`, see the README) and stop.
2. Note the version: `tars --version`.
3. Run `tars update`. It upgrades the CLI and the app through Homebrew (whichever are installed), then the plugin, and keeps going past a part that fails. It never touches the dev build (`tars-dev`, Tars Dev, the tars-dev plugin).
   - If it says `unknown command "update"`, this tars predates it: run `brew update && brew upgrade --cask tars` once, then `tars update`.
4. Run `tars --version` again.

Report in a few lines: the CLI version before and after, what was skipped and why (as `tars update` printed it), and what the user has to do:
- restart Claude Code (or run `/reload-plugins`) to load a new plugin version
- quit and reopen Tars if the app was upgraded while open
