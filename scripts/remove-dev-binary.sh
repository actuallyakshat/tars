#!/usr/bin/env bash
# Remove the tars binary that `go install ./cmd/tars` put in your Go bin
# directory, so the Homebrew-installed tars is the one on your PATH.
#
# Only deletes a binary that was built from this module, unless --force.
#
#   scripts/remove-dev-binary.sh          # remove it
#   scripts/remove-dev-binary.sh --force  # remove even if it wasn't built from this repo
set -euo pipefail

module="github.com/actuallyakshat/tars"
force=false
[[ "${1:-}" == "--force" ]] && force=true

bin_dir=""
if command -v go >/dev/null 2>&1; then
  bin_dir="$(go env GOBIN)"
  [[ -z "$bin_dir" ]] && bin_dir="$(go env GOPATH)/bin"
fi
[[ -z "$bin_dir" ]] && bin_dir="$HOME/go/bin"

target="$bin_dir/tars"
if [[ ! -e "$target" ]]; then
  echo "No Go-installed tars at $target. Nothing to remove."
else
  if ! $force; then
    built_from="$(go version -m "$target" 2>/dev/null | awk '$1 == "path" { print $2 }')"
    if [[ "$built_from" != "$module/cmd/tars" ]]; then
      echo "Refusing to remove $target: it wasn't built from $module (found: ${built_from:-unknown})." >&2
      echo "Re-run with --force to remove it anyway." >&2
      exit 1
    fi
  fi
  rm "$target"
  echo "Removed $target"
fi

hash -r 2>/dev/null || true
if now="$(command -v tars)"; then
  echo "tars on your PATH is now: $now"
else
  echo "No tars left on your PATH. Install it with:"
  echo "  brew tap actuallyakshat/tap"
  echo "  brew trust --cask actuallyakshat/tap/tars"
  echo "  brew install --cask tars"
fi
