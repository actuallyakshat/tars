import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App, REFRESH_MS } from "./App";
import { fakeBackend, sampleTasks } from "./test/backend";

const views = ["Needs you", "Open", "Inbox", "Ready", "Review", "Blocked", "Done"];

async function boot() {
  const user = userEvent.setup();
  render(<App />);
  await screen.findByRole("navigation", { name: "Views" });
  return user;
}

const sidebar = () => within(screen.getByRole("navigation", { name: "Views" }));
const count = (i: number) => screen.getByTestId(`count-${i}`).textContent;
const detail = () => within(screen.getByRole("region", { name: "Task details" }));
const heading = () => screen.getByRole("heading", { level: 1 }).textContent;

describe("sidebar", () => {
  it("shows every view with its count and opens on Needs you when an agent is waiting", async () => {
    fakeBackend();
    await boot();
    expect(views.map((_, i) => count(i))).toEqual(["1", "4", "0", "1", "1", "1", "1"]);
    for (const v of views) expect(sidebar().getByRole("button", { name: new RegExp(`^${v}`) })).toBeInTheDocument();
    expect(heading()).toBe("Needs you");
    expect(sidebar().getByRole("button", { name: /^Needs you/ })).toHaveAttribute("aria-current", "page");
  });

  it("opens on Open when nothing needs you", async () => {
    fakeBackend(sampleTasks().filter((t) => !t.needs_you));
    await boot();
    expect(heading()).toBe("Open");
    expect(count(0)).toBe("0");
  });

  it("marks views holding agent updates with a dot, except Done", async () => {
    fakeBackend();
    await boot();
    expect(sidebar().getByRole("button", { name: /^Open/ })).toHaveTextContent("●");
    expect(sidebar().getByRole("button", { name: /^Inbox/ })).not.toHaveTextContent("●");
  });

  it("switches views by number key", async () => {
    fakeBackend();
    const user = await boot();
    await user.keyboard("6");
    expect(heading()).toBe("Blocked");
    expect(screen.getByTestId("row-2")).toHaveTextContent("waiting on Rahul");
  });
});

describe("task list", () => {
  it("groups by priority and shows the markers", async () => {
    fakeBackend();
    const user = await boot();
    await user.keyboard("2");
    const groups = screen.getAllByRole("listbox").map((l) => l.getAttribute("aria-label"));
    expect(groups).toEqual(["Urgent priority", "High priority", "Normal priority", "Low priority"]);

    const asking = screen.getByTestId("row-1");
    expect(asking).toHaveTextContent("?");
    expect(asking).toHaveClass("unread");
    expect(asking).toHaveTextContent("#1");
    expect(asking).toHaveTextContent("BILL-881");
    expect(asking).toHaveTextContent("billing-svc");
    expect(asking).toHaveTextContent("tomorrow");
    expect(asking).toHaveTextContent("▲");

    const unread = screen.getByTestId("row-3");
    expect(unread).toHaveTextContent("●");
    expect(unread).toHaveClass("unread");
    expect(screen.getByTestId("row-2")).not.toHaveClass("unread");
    expect(screen.getByTestId("row-4")).toHaveTextContent("2d late");
    expect(screen.queryByTestId("row-5")).toBeNull(); // done stays out of Open
  });

  it("moves with j/k and keeps the selection across refreshes", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const b = fakeBackend();
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      render(<App />);
      await screen.findByRole("navigation", { name: "Views" });
      await user.keyboard("2");
      expect(screen.getByTestId("row-2")).toHaveAttribute("aria-selected", "true");
      await user.keyboard("jj");
      expect(screen.getByTestId("row-4")).toHaveAttribute("aria-selected", "true");
      await user.keyboard("k");
      expect(screen.getByTestId("row-1")).toHaveAttribute("aria-selected", "true");

      // An agent adds an urgent task, which lands above the selection.
      b.tasks.push({ id: 6, title: "Hotfix login", status: "inbox", priority: 0, created_at: "", updated_at: "", unread: true });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(REFRESH_MS + 10);
      });
      expect(await screen.findByTestId("row-6")).toBeInTheDocument();
      expect(screen.getByTestId("row-1")).toHaveAttribute("aria-selected", "true");
      expect(count(1)).toBe("5");
    } finally {
      vi.useRealTimers();
    }
  });

  it("shows empty states", async () => {
    fakeBackend([]);
    const user = await boot();
    expect(screen.getByText("No open tasks. Press c to add one.")).toBeInTheDocument();
    await user.keyboard("1");
    expect(screen.getByText(/Nothing needs you/)).toBeInTheDocument();
    expect(detail().getByText(/Select a task/)).toBeInTheDocument();
  });
});

describe("detail pane", () => {
  it("shows the report card, fields, sessions and a collapsed activity log", async () => {
    const b = fakeBackend();
    const user = await boot();
    const d = detail();
    expect(await d.findByText("Backoff, 5 tries")).toBeInTheDocument();
    expect(d.getByRole("heading", { name: "Add retry to webhook sender" })).toBeInTheDocument();
    const report = within(d.getByRole("region", { name: "Report" }));
    expect(report.getByText("make test: 212 passed")).toBeInTheDocument();
    expect(report.getByText("Alert #oncall or #billing-alerts?")).toBeInTheDocument();
    expect(report.getByText("Open PR")).toBeInTheDocument();
    expect(d.getByRole("combobox", { name: "Status" })).toHaveValue("in-progress");
    expect(d.getByText("▲ High")).toBeInTheDocument();
    expect(d.getByText("Webhooks drop on 502.")).toBeInTheDocument();
    expect(d.getByText("Retries 5 times with backoff")).toBeInTheDocument();
    expect(d.getByText("#backend")).toBeInTheDocument();

    expect(d.getByText("webhook retry")).toBeInTheDocument();
    await user.click(d.getByRole("button", { name: "Copy resume command" }));
    expect(b.callsTo("copy_text")).toEqual([{ text: "claude --resume sess-42" }]);
    await user.click(d.getByRole("button", { name: "Open in Terminal" }));
    expect(b.callsTo("open_in_terminal")).toEqual([{ sessionId: "sess-42", launchDir: "/work/billing" }]);

    await user.click(d.getByRole("button", { name: "tars/1-webhook-retry" }));
    expect(b.callsTo("copy_text")[1]).toEqual({ text: "tars/1-webhook-retry" });
    await user.click(d.getByRole("button", { name: "/work/billing-1" }));
    expect(b.callsTo("reveal_path")).toEqual([{ path: "/work/billing-1" }]);

    const toggle = d.getByRole("button", { name: /Activity/ });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(toggle).toHaveTextContent("2 entries · last: agent · report");
    expect(d.queryByText("Started looking")).toBeNull();
    await user.keyboard("l");
    expect(d.getByText("Started looking")).toBeInTheDocument();
  });

  it("copies the Jira key, or opens it once a Jira URL is set", async () => {
    const b = fakeBackend();
    const user = await boot();
    await user.click(await detail().findByRole("button", { name: "BILL-881" }));
    expect(b.callsTo("copy_text")).toEqual([{ text: "BILL-881" }]);
    expect(await screen.findByRole("status")).toHaveTextContent("Add your Jira URL in Settings");

    await user.click(sidebar().getByRole("button", { name: "Settings" }));
    await user.type(screen.getByRole("textbox", { name: /Jira URL/ }), "https://acme.atlassian.net/");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await user.click(detail().getByRole("button", { name: "BILL-881" }));
    expect(b.callsTo("open_url")).toEqual([{ url: "https://acme.atlassian.net/browse/BILL-881" }]);
  });
});

describe("actions", () => {
  it("answers the open question through the CLI", async () => {
    const b = fakeBackend();
    const user = await boot();
    await detail().findByText("Backoff, 5 tries");
    await user.keyboard("a");
    const box = detail().getByRole("textbox", { name: "Your answer" });
    expect(box).toHaveFocus();
    await user.type(box, "billing-alerts{Enter}");

    expect(b.callsTo("answer_task")).toEqual([{ id: 1, text: "billing-alerts" }]);
    expect(await screen.findByRole("status")).toHaveTextContent("Answered #1");
    await waitFor(() => expect(count(0)).toBe("0"));
    // The answered task leaves Needs you; the asked and answered question stays on the task.
    expect(screen.getByText(/Nothing needs you/)).toBeInTheDocument();
    await user.keyboard("2");
    await user.click(screen.getByTestId("row-1"));
    const report = within(await detail().findByRole("region", { name: "Report" }));
    expect(report.getByText("Alert #oncall or #billing-alerts?")).toBeInTheDocument();
    expect(report.getByText("billing-alerts")).toBeInTheDocument();
    expect(report.queryByRole("button", { name: "Answer" })).toBeNull();
  });

  it("marks done with x, reopens, and changes status from the picker", async () => {
    const b = fakeBackend();
    const user = await boot();
    await user.keyboard("2");
    await user.click(screen.getByTestId("row-3"));
    await user.keyboard("x");
    expect(b.callsTo("set_status")).toEqual([{ id: 3, status: "done" }]);
    await waitFor(() => expect(count(6)).toBe("2"));
    expect(screen.queryByTestId("row-3")).toBeNull();
    // The selection moves to the task that took its place.
    await waitFor(() => expect(screen.getByTestId("row-4")).toHaveAttribute("aria-selected", "true"));

    await user.keyboard("7");
    await user.click(screen.getByTestId("row-3"));
    await user.click(detail().getByRole("button", { name: "Reopen" }));
    expect(b.callsTo("set_status")[1]).toEqual({ id: 3, status: "inbox" });
    await waitFor(() => expect(count(2)).toBe("1"));

    await user.keyboard("2");
    await user.click(screen.getByTestId("row-4"));
    await user.selectOptions(detail().getByRole("combobox", { name: "Status" }), "blocked");
    expect(b.callsTo("set_status")[2]).toEqual({ id: 4, status: "blocked" });
    await waitFor(() => expect(count(5)).toBe("2"));
  });

  it("adds a note, and typing in the note box doesn't trigger shortcuts", async () => {
    const b = fakeBackend();
    const user = await boot();
    await detail().findByText("Backoff, 5 tries");
    await user.keyboard("n");
    const box = detail().getByRole("textbox", { name: "Note on #1" });
    await user.type(box, "x marks the spot 2{Enter}");
    expect(b.callsTo("set_status")).toEqual([]);
    expect(heading()).toBe("Needs you");
    expect(b.callsTo("add_note")).toEqual([{ id: 1, text: "x marks the spot 2" }]);
    expect(await screen.findByRole("status")).toHaveTextContent("Noted on #1");
  });

  it("adds a task from the new task sheet", async () => {
    const b = fakeBackend();
    const user = await boot();
    await user.keyboard("2c");
    const sheet = within(screen.getByRole("dialog", { name: "New task" }));
    await user.type(sheet.getByRole("textbox", { name: "Title" }), "Write release notes");
    await user.selectOptions(sheet.getByRole("combobox", { name: "Priority" }), "high");
    await user.type(sheet.getByRole("textbox", { name: "Jira" }), "DOC-12");
    await user.click(sheet.getByRole("button", { name: "Add task" }));

    expect(b.callsTo("add_task")).toEqual([
      { task: { title: "Write release notes", priority: "high", jira: "DOC-12" } },
    ]);
    expect(await screen.findByTestId("row-6")).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows CLI errors from actions without leaving the app", async () => {
    const b = fakeBackend();
    const user = await boot();
    await detail().findByText("Backoff, 5 tries");
    b.fail.answer_task = { kind: "failed", message: "#1 has no open question" };
    await user.keyboard("a");
    await user.type(detail().getByRole("textbox", { name: "Your answer" }), "yes{Enter}");
    expect(await screen.findByRole("alert")).toHaveTextContent("#1 has no open question");
    expect(screen.getByRole("navigation", { name: "Views" })).toBeInTheDocument();
  });
});

describe("CLI problems", () => {
  it("explains a missing CLI and retries", async () => {
    const b = fakeBackend();
    b.fail.cli_status = { kind: "missing", message: "Couldn't find the tars CLI. Set TARS_BIN." };
    const user = userEvent.setup();
    render(<App />);
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("tars needs its command-line tool");
    expect(alert).toHaveTextContent("Couldn't find the tars CLI");

    delete b.fail.cli_status;
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("navigation", { name: "Views" })).toBeInTheDocument();
  });

  it("shows other load failures with a retry", async () => {
    const b = fakeBackend();
    b.fail.list_tasks = { kind: "failed", message: "open /nope/tars.db: permission denied" };
    const user = userEvent.setup();
    render(<App />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Couldn't load tasks: open /nope/tars.db");
    delete b.fail.list_tasks;
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("navigation", { name: "Views" })).toBeInTheDocument();
  });

  it("explains an outdated CLI", async () => {
    const b = fakeBackend();
    b.fail.list_tasks = { kind: "too_old", message: "/opt/homebrew/bin/tars is too old for this app" };
    render(<App />);
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Your tars CLI needs an update");
    expect(alert).toHaveTextContent("/opt/homebrew/bin/tars is too old");
  });
});
