#!/usr/bin/env bash
# Cut a tars release: bump the version of the CLI, the desktop app and the
# Claude Code plugin, commit, tag, and (after you confirm) push.
#
#   scripts/release.sh 0.4.0 "Usage page, todo lists, tars update"
#   scripts/release.sh 0.4.0 "..." --yes     # push without asking
#
# Pushing the tag is what ships it. .github/workflows/release.yml then:
#   - CLI:     goreleaser builds every platform, publishes the GitHub release
#              and updates the `tars` cask in actuallyakshat/homebrew-tap
#   - Desktop: builds the universal Tars.app, attaches it to the release and
#              updates the `tars-app` cask
#   - Plugin:  installs straight from this repo's marketplace, so the bumped
#              plugin/.claude-plugin/plugin.json on main is the release
#
# The CLI's version comes from the tag (ldflags), so it has no file to bump.
set -euo pipefail

usage() { sed -n '2,6p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 1; }

version="${1:-}"; summary="${2:-}"; yes=false
[[ "${3:-}" == "--yes" ]] && yes=true
[[ -z "$version" || -z "$summary" ]] && usage
version="${version#v}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Version must look like 1.2.3, got: $version" >&2; exit 1; }
tag="v$version"

cd "$(git rev-parse --show-toplevel)"

# Preconditions: a clean main that matches origin, and a tag that's free.
branch="$(git rev-parse --abbrev-ref HEAD)"
[[ "$branch" == "main" ]] || { echo "Release from main (you're on $branch)." >&2; exit 1; }
[[ -z "$(git status --porcelain)" ]] || { echo "Working tree isn't clean; commit or stash first." >&2; exit 1; }
git fetch --quiet --tags origin main
[[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || {
  echo "main isn't in sync with origin/main; pull or push first." >&2; exit 1; }
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
  echo "Tag $tag already exists." >&2; exit 1
fi

current="$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' plugin/.claude-plugin/plugin.json | head -1)"
echo "Releasing $current -> $version"

# Bump every place the version lives.
set_json_version() { # file: replaces the first top-level "version" field
  perl -0pi -e 's/("version":\s*")[^"]*(")/${1}'"$version"'${2}/' "$1"
}
set_json_version plugin/.claude-plugin/plugin.json
set_json_version desktop/src-tauri/tauri.conf.json
(cd desktop && npm version "$version" --no-git-tag-version --allow-same-version >/dev/null) # package.json + lock
perl -0pi -e 's/(\[package\]\nname = "tars-desktop"\nversion = ")[^"]*(")/${1}'"$version"'${2}/' desktop/src-tauri/Cargo.toml
perl -0pi -e 's/(name = "tars-desktop"\nversion = ")[^"]*(")/${1}'"$version"'${2}/' desktop/src-tauri/Cargo.lock

# Check every bump landed.
files=(plugin/.claude-plugin/plugin.json desktop/src-tauri/tauri.conf.json desktop/package.json
       desktop/package-lock.json desktop/src-tauri/Cargo.toml desktop/src-tauri/Cargo.lock)
changed="$(git diff --name-only | sort)"
expected="$(printf '%s\n' "${files[@]}" | sort)"
if [[ "$changed" != "$expected" ]]; then
  echo "Version bump didn't touch exactly the expected files." >&2
  echo "Expected:" >&2; echo "$expected" >&2
  echo "Changed:" >&2;  echo "$changed" >&2
  git checkout -- . ; exit 1
fi
git --no-pager diff --stat

git add "${files[@]}"
git commit --quiet -m "Release $version: $summary"
git tag -a "$tag" -m "$tag: $summary"
echo "Committed and tagged $tag."

push=(git push --atomic origin main "$tag")
if ! $yes; then
  read -r -p "Push main and $tag to origin now? This publishes the release. [y/N] " answer
  [[ "$answer" =~ ^[Yy]$ ]] || {
    echo "Not pushed. When ready: ${push[*]}"
    echo "To undo instead: git tag -d $tag && git reset --hard HEAD~1"
    exit 0
  }
fi
"${push[@]}"
echo "Pushed. Watch the build: gh run watch \$(gh run list --workflow release.yml --limit 1 --json databaseId -q '.[0].databaseId')"
