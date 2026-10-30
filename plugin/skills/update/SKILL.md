---
name: update
description: Update TARS itself to the latest release: the tars CLI, this Claude Code plugin, and the Tars desktop app for macOS. Reports the versions before and after.
argument-hint: "[optional: cli | plugin | app]"
disable-model-invocation: true
allowed-tools: Bash(tars --version), Bash(brew update), Bash(brew info:*), Bash(brew list:*), Bash(brew outdated:*), Bash(brew upgrade:*), Bash(claude plugin marketplace update:*), Bash(claude plugin update:*), Bash(claude plugin list:*), Bash(command -v:*), Bash(pgrep:*), Bash(defaults read:*), Bash(mdfind:*)
---

# /tars:update

Only update (may be empty, meaning all three): $ARGUMENTS

Update the parts that are already installed. **Never install something that isn't there without asking first**, and never touch a source checkout or a dev build. Work through the three parts in order and keep going if one fails; report each at the end.

## 1. The CLI

1. Note the current version: `tars --version` (or "not installed" if `command -v tars` fails).
2. Find where it came from:
   - `brew list --cask tars` succeeds → `brew update`, then `brew upgrade --cask tars`. "Already up-to-date" is a fine outcome.
   - `command -v tars` points into a Go bin directory (`~/go/bin`, `$GOBIN`) → it's a local build. Don't touch it. Say so, and that `scripts/remove-dev-binary.sh` in the tars repo switches back to the Homebrew build.
   - Not on macOS or no Homebrew → point to https://github.com/actuallyakshat/tars/releases/latest. Don't download anything.
   - Not installed → ask whether to run `brew install --cask actuallyakshat/tap/tars` before doing it.
3. Note the version after: `tars --version`.

## 2. The plugin

1. `claude plugin marketplace update tars` refreshes the catalog from GitHub.
2. `claude plugin update tars@tars` installs the newer plugin, if there is one.
3. The new skills load only after a reload: tell the user to run `/reload-plugins` (or restart Claude Code). You can't run slash commands yourself.

## 3. The desktop app (macOS only)

The app ships as the `tars-app` Homebrew cask (a universal, unsigned `Tars.app`).

1. Check how it's installed:
   - `brew list --cask tars-app` succeeds → `brew upgrade --cask tars-app`.
   - A Tars process is running from a source checkout (`pgrep -fl "tauri dev"`, or a `Tars` process under `src-tauri/target`) → it's a development copy. Don't touch it. Tell the user to `git pull` and `npm install` in that checkout's `desktop/` folder.
   - Neither → ask whether to install it with `brew install --cask actuallyakshat/tap/tars-app`.
2. Homebrew replaces the app while it runs, but the open window keeps the old code. If it's running, tell the user to quit and reopen Tars.
3. Read the installed version with `defaults read /Applications/Tars.app/Contents/Info CFBundleShortVersionString`.

The app needs a CLI new enough for it. If the app updated and the CLI couldn't be (a dev build, say), mention that the two may disagree.

## Report

End with a short table, one row per part, and leave out parts that weren't asked for:

| Part | Before | After | Note |
|---|---|---|---|
| CLI | 0.2.0 | 0.3.0 | brew |
| Plugin | 0.2.0 | 0.3.0 | run `/reload-plugins` |
| Desktop app | 0.2.0 | 0.3.0 | quit and reopen Tars |

Then list anything the user has to do: reload plugins, reopen the app, answer an install question. No task in the ledger changes, so there's no `tars report` here.
