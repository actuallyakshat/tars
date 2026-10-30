---
name: sweep
description: Pick ready TARS tasks and work each one in its own git worktree. Implement it, write tests, run the repo's test suite and commit locally on a branch for the user to review. Never pushes.
argument-hint: "[filter, e.g. 'jira only', 'billing-svc', '#12 #14', 'max 2'] [--yes]"
disable-model-invocation: true
---

# /tars:sweep: Run ready tasks into reviewed-ready branches

The user asked for a sweep with these arguments (may be empty):

> $ARGUMENTS

You will take **ready** tasks from the `tars` ledger and, for each one, produce a local branch with the implementation and passing tests. The user reviews these in the morning, typically with `/tars:brief`.

## Hard rules

- **Never push, open PRs, merge, rebase shared branches, or force anything.** Everything stays local: a new branch in a new worktree.
- **Never touch the user's main checkout.** All edits happen inside the task's worktree.
- **Never run destructive commands** (`rm -rf` outside the worktree, `git reset --hard` on existing branches, dropping databases, migrations against non-local DBs).
- **Only connect to local databases.** If the repo profile's `db` section points at a shared or remote environment, don't write to it.
- **Precedence:** the repo's CLAUDE.md / AGENTS.md beat the tars repo profile, which beats the personal profile.
- **Log everything with `--by agent`,** so the user can audit it in `tars show <id>`.
- If a requirement is ambiguous, **don't guess.** Block the task with a question (step 4) and move on.

## 1. Select tasks

1. Get candidates with `tars ls -s ready --json`.
2. Apply the user's filter from the arguments:
   - "jira only" → keep tasks with a Jira key
   - a repo name → that repo
   - `#12 #14` → exactly those IDs (they must still be `ready`)
   - "max N" → cap the count
   - default cap: 5 tasks
3. For each candidate, load `tars repo show <repo> --json`. **Skip** any task (and say why) when:
   - it has no repo, or the repo profile has no `path`
   - the path isn't a git repo
   - the task has no acceptance criteria
4. If nothing is left, say so and suggest `/tars triage my inbox`. Then stop.

## 2. Confirm the plan

Show a table with the columns: `#id | title | repo | base branch | new branch | test command`.

- **Base branch:** the repo's default branch (`git -C <path> symbolic-ref --short refs/remotes/origin/HEAD`, stripped of `origin/`). If that fails, use the currently checked-out branch.
- **New branch:** `tars/<id>-<short-slug-of-title>`.
- **Test command:** from the repo profile's `tests` section or the repo's CLAUDE.md. If you can't find one, put "unknown — will discover" in the column.

Unless the arguments contain `--yes`, ask the user to confirm (AskUserQuestion if available) before doing anything. This is the only time you wait for them. After confirmation, run unattended.

## 3. Set up each task

For each task, in order:

```bash
tars mv <id> in-progress --by agent
git -C <repo-path> worktree add -b tars/<id>-<slug> <repo-path>/../<repo-name>.tars-<id> <base>
tars edit <id> --branch tars/<id>-<slug> --worktree <absolute-worktree-path> --by agent
```

If the branch or worktree already exists from an earlier sweep, reuse it. Read the existing log (`tars show <id> --json`) and continue rather than starting over.

## 4. Work each task

Up to 3 tasks can run at once, each delegated to a subagent (Agent tool) with its own worktree. If subagents aren't available, work through them one by one. Give each subagent this brief, filled in:

> You are implementing TARS task #<id> in the git worktree `<worktree>`. Work only inside that directory.
>
> 1. Run `tars context <id>` and read it fully (personal profile, repo profile, task, log). Then read CLAUDE.md / AGENTS.md in the worktree. They take precedence over the tars context.
> 2. If the task has a Jira key and Jira tools are available, read the issue (read-only) for extra detail.
> 3. Study the relevant code and the existing tests to learn the test conventions. Follow the repo profile's `tests` and `conventions` sections.
> 4. Write tests that pin down every acceptance criterion. Write them before or alongside the implementation.
> 5. Implement the change, keeping it as small as the criteria allow.
> 6. Run the targeted tests, then the repo's full relevant test command. Fix and re-run, up to 3 fix cycles. Run the linter or formatter too, if the repo has one.
> 7. Check every item in the repo profile's `dod` section.
> 8. If everything passes, commit on the current branch. Follow the repo's commit conventions and the personal profile's `commits` section, and include the Jira key if there is one. Do not push.
> 9. Never push, never touch other checkouts, never write to non-local databases.
> 10. Return: status (`success` | `failed` | `needs-input`), a summary of the change, files touched, tests added, the test command with a short result (pass/fail counts), DoD checklist results, the commit SHA, and any open questions.

## 5. Record the outcome

Write the subagent's report to the task log:

```bash
tars note <id> --by agent -k sweep - <<'EOF'
## Sweep result: <status>
**Change:** …
**Tests added:** …
**Test run:** `<command>` → <N passed, M failed>
**DoD:** ✔ … / ✘ …
**Commit:** <sha> on tars/<id>-<slug>
**Open questions:** …
EOF
```

Then move the task according to the result:

- **success** (tests pass, DoD met, committed): `tars mv <id> in-review --by agent`
- **needs-input:** `tars block <id> --on "you: <the question, one line>" --by agent`
- **failed:** leave it `in-progress`. The note must explain what failed and what to try next.

## 6. Learnings (optional, ask first)

If you discovered reusable repo facts during the sweep, list them as proposed additions to the repo profile's `tests`, `setup` or `gotchas` sections. Examples: "integration tests need `docker compose up db`", or "`make test` skips e2e; use `make test-all`". Save them with `tars repo set` **only if the user approves**. If the sweep ran with `--yes`, don't save them. Leave them in the final report instead.

## 7. Final report

Finish with:

- a table of `#id | title | result | branch | tests`
- for each blocked task, its question
- proposed profile learnings, if any
- one line: "Review with `/tars:brief`, or `tars show <id>` for details. Worktrees are at `<repo>/../<name>.tars-<id>`."
