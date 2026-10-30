---
name: park
description: Save where this session got to on a TARS task (decisions, dead ends, next step) so you or an agent can resume it tomorrow with /tars resume <id>.
argument-hint: "[task id] [optional note]"
disable-model-invocation: true
allowed-tools: Bash(tars:*), Bash(git status:*), Bash(git branch:*), Bash(git rev-parse:*), Bash(git log:*), Bash(git diff --stat:*)
---

# /tars:park

Arguments: $ARGUMENTS

Capture the state of the current session on a task so that work can resume later.

1. **Work out which task this is.**
   - Use the ID from the arguments if one is given.
   - Otherwise, if the current branch looks like `tars/<id>-…`, use that ID.
   - Otherwise, search with `tars ls -q <keyword> --json` using terms from this conversation.
   - If it's still ambiguous, ask the user. If no task exists, offer to create one with `tars add`.
2. **Gather state:**
   - `git rev-parse --show-toplevel`, the current branch, `git status --short`, `git diff --stat`, and `git log --oneline -5`.
   - From this conversation: the goal, what's done, decisions made and why, approaches that failed, what's left, and the exact next step.
3. **Save it** as a park note:

   ```bash
   tars note <id> --by agent -k park - <<'EOF'
   ## Parked
   **Goal:** …
   **Done so far:** …
   **Decisions:** … (why)
   **Dead ends:** … (don't retry: why)
   **Next step:** <one concrete action>
   **Open questions:** …
   **State:** branch <b>, <n> uncommitted files: …
   EOF
   ```

4. If the branch or worktree isn't recorded on the task yet, run `tars edit <id> --branch <b> --worktree <toplevel> --by agent`.
5. Don't change the status unless the user says so. Parking isn't blocking.
6. Confirm in one line: "Parked #<id>. Resume with `/tars:tars resume <id>`."

Don't commit, stash, or discard anything. Parking only records state.
