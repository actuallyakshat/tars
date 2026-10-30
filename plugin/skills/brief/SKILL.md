---
name: brief
description: Morning briefing from TARS. Covers what the last sweep produced, what's waiting for your review, what's overdue or due today, follow-ups to send, and stale work, then suggests a focus for the day.
argument-hint: "[optional focus, e.g. 'just reviews']"
disable-model-invocation: true
allowed-tools: Bash(tars:*), Bash(git log:*), Bash(git diff --stat:*)
---

# /tars:brief

Focus (may be empty): $ARGUMENTS

This command is **read-only**. Don't change any task unless the user asks afterwards.

1. Run `tars brief --json`.
2. For each task in `in_review`, run `tars show <id> --json`. Take the most recent `sweep` log entry. If the task has a worktree and branch, run `git -C <worktree> log --oneline -3` and `git -C <worktree> diff --stat <base>...HEAD` to show the size of the change. Use `--stat` only, never the full diff.
3. Write the brief in this order, skipping empty sections:
   - **Review queue:** for each task, the ID, title, branch, a one-line summary of the change, the test result, and any open questions from the sweep note.
   - **Needs your answer:** blocked tasks where `waiting_on` starts with "you:". These are questions a sweep left for the user.
   - **Overdue / due today**
   - **Follow-ups:** blocked tasks whose follow-up date has arrived. Draft a one-line nudge for each person.
   - **Stale:** in-progress tasks untouched for 3+ days.
   - **Queue:** a count of tasks that are ready, plus a count of inbox tasks that need triage.
4. Close with **Suggested focus:** at most 3 items for today, picked by priority, due date, and how close each is to done. Reviews are usually fastest to close.

Keep it short enough to read in a minute. Reference tasks as `#id`.
