import { DONE, OPEN, ago, counts, defaultView, dueText, flatten, groupTasks, jiraUrl, openQuestion } from "./model";
import { sampleTasks } from "./test/backend";

const now = new Date(2026, 9, 30, 12, 0, 0); // Fri Oct 30 2026

describe("model", () => {
  it("counts views like the TUI: Review is in-review only, Open is everything not done", () => {
    expect(counts(sampleTasks()).map((c) => c.total)).toEqual([1, 4, 0, 1, 1, 1, 1]);
    expect(counts(sampleTasks()).map((c) => c.unread)).toEqual([1, 2, 0, 1, 0, 0, 0]);
  });

  it("opens on Needs you only when it has tasks", () => {
    expect(defaultView(sampleTasks())).toBe(0);
    expect(defaultView(sampleTasks().map((t) => ({ ...t, needs_you: false })))).toBe(OPEN);
  });

  it("orders by priority, then due date (undated last), then id", () => {
    const base = { status: "inbox" as const, created_at: "", updated_at: "" };
    const tasks = [
      { ...base, id: 1, title: "a", priority: 2 },
      { ...base, id: 2, title: "b", priority: 2, due: "2026-11-02" },
      { ...base, id: 3, title: "c", priority: 0 },
      { ...base, id: 4, title: "d", priority: 2, due: "2026-11-01" },
    ];
    const groups = groupTasks(tasks, OPEN);
    expect(groups.map((g) => g.label)).toEqual(["Urgent", "Normal"]);
    expect(flatten(groups).map((t) => t.id)).toEqual([3, 4, 2, 1]);
  });

  it("puts the most recently finished first in Done", () => {
    const base = { status: "done" as const, priority: 2, created_at: "", updated_at: "" };
    const tasks = [
      { ...base, id: 1, title: "a", done_at: "2026-10-01T00:00:00Z" },
      { ...base, id: 2, title: "b", done_at: "2026-10-03T00:00:00Z" },
    ];
    expect(flatten(groupTasks(tasks, DONE)).map((t) => t.id)).toEqual([2, 1]);
  });

  it("describes due dates", () => {
    expect(dueText("2026-10-28", now)).toEqual({ text: "2d late", tone: "late" });
    expect(dueText("2026-10-30", now)).toEqual({ text: "today", tone: "today" });
    expect(dueText("2026-10-31", now)).toEqual({ text: "tomorrow", tone: "soon" });
    expect(dueText("2026-11-02", now)).toEqual({ text: "Mon", tone: "muted" });
    expect(dueText("2026-11-20", now)).toEqual({ text: "Nov 20", tone: "muted" });
    expect(dueText(undefined, now).text).toBe("");
  });

  it("describes times", () => {
    expect(ago(new Date(now.getTime() - 30_000).toISOString(), now)).toBe("just now");
    expect(ago(new Date(now.getTime() - 5 * 60_000).toISOString(), now)).toBe("5m ago");
    expect(ago(new Date(now.getTime() - 3 * 3600_000).toISOString(), now)).toBe("3h ago");
    expect(ago(new Date(now.getTime() - 2 * 86400_000).toISOString(), now)).toBe("2d ago");
  });

  it("finds open questions and builds Jira links", () => {
    const [asking] = sampleTasks();
    expect(openQuestion(asking)).toBe("Alert #oncall or #billing-alerts?");
    expect(openQuestion({ ...asking, report: { ...asking.report!, answered_at: "x" } })).toBe("");
    expect(jiraUrl("https://acme.atlassian.net/", "BILL-881")).toBe("https://acme.atlassian.net/browse/BILL-881");
    expect(jiraUrl("", "BILL-881")).toBe("");
  });
});
