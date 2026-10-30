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
    const b = fakeBackend();
    const user = await boot();
    await user.keyboard("2");
    const groups = screen.getAllByRole("listbox").map((l) => l.getAttribute("aria-label"));
    expect(groups).toEqual(["Urgent", "High", "Normal", "Low"]);

    const asking = screen.getByTestId("row-1");
    expect(asking).toHaveTextContent("?");
    // #1 starts selected, and the detail pane counts as looking at it.
    await waitFor(() => expect(b.callsTo("mark_seen")).toEqual([{ id: 1 }]));
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

  it("marks a task seen when it is selected, clearing its ●", async () => {
    const b = fakeBackend();
    const user = await boot();
    await user.keyboard("2");
    const row = screen.getByTestId("row-3");
    expect(row).toHaveTextContent("●");
    expect(row).toHaveClass("unread");

    await user.click(row);
    await waitFor(() => expect(b.callsTo("mark_seen")).toContainEqual({ id: 3 }));
    await waitFor(() => expect(screen.getByTestId("row-3")).not.toHaveClass("unread"));
    expect(screen.getByTestId("row-3")).not.toHaveTextContent("●");

    // Selecting a task that was already seen doesn't call the CLI again.
    const before = b.callsTo("mark_seen").length;
    await user.click(screen.getByTestId("row-2"));
    await waitFor(() => expect(screen.getByTestId("row-2")).toHaveAttribute("aria-selected", "true"));
    expect(b.callsTo("mark_seen")).toHaveLength(before);
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
    expect(d.getByRole("button", { name: "Status: In progress" })).toBeInTheDocument();
    expect(d.getByRole("button", { name: "Priority: High" })).toBeInTheDocument();
    expect(d.getByText("Webhooks drop on 502.")).toBeInTheDocument();
    expect(d.getByText("Retries 5 times with backoff")).toBeInTheDocument();
    expect(d.getByText("#backend")).toBeInTheDocument();

    expect(d.getByText("webhook retry")).toBeInTheDocument();
    await user.click(d.getByRole("button", { name: "Copy command" }));
    expect(b.callsTo("copy_text")).toEqual([{ text: "claude --resume sess-42" }]);
    await user.click(d.getByRole("button", { name: "Resume in Terminal" }));
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
    const settings = within(screen.getByRole("main", { name: "Settings" }));
    await user.click(settings.getByRole("button", { name: "Integrations" }));
    await user.type(settings.getByRole("textbox", { name: /Jira URL/ }), "https://acme.atlassian.net/");
    await user.click(settings.getByRole("button", { name: "Save" }));
    expect(settings.getByText("Saved")).toBeInTheDocument();
    await user.click(settings.getByRole("button", { name: "Back" }));
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
    await user.click(detail().getByRole("button", { name: /^Status:/ }));
    await user.click(within(screen.getByRole("listbox", { name: "Status" })).getByRole("option", { name: "Blocked" }));
    expect(b.callsTo("set_status")[2]).toEqual({ id: 4, status: "blocked" });
    await waitFor(() => expect(count(5)).toBe("2"));
    expect(screen.queryByRole("listbox", { name: "Status" })).toBeNull();

    // From the keyboard: s opens the menu, arrows move, Enter chooses.
    await user.keyboard("s");
    const menu = await screen.findByRole("listbox", { name: "Status" });
    expect(within(menu).getByRole("option", { name: "Blocked" })).toHaveAttribute("aria-selected", "true");
    await user.keyboard("{ArrowDown}{Enter}");
    expect(b.callsTo("set_status")[3]).toEqual({ id: 4, status: "done" });

    // Esc closes the menu without changing anything.
    await user.keyboard("2");
    await user.click(screen.getByTestId("row-2"));
    await user.click(detail().getByRole("button", { name: /^Status:/ }));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox", { name: "Status" })).toBeNull();
    expect(b.callsTo("set_status")).toHaveLength(4);
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

  it("adds a task from the new task page", async () => {
    const b = fakeBackend();
    const user = await boot();
    await user.keyboard("2");
    await user.click(sidebar().getByRole("button", { name: /New task/ }));
    const page = within(screen.getByRole("main", { name: "New task" }));
    expect(page.getByRole("textbox", { name: "Title" })).toHaveFocus();
    expect(page.getByRole("button", { name: /Create task/ })).toBeDisabled();
    await user.type(page.getByRole("textbox", { name: "Title" }), "Write release notes");
    await user.click(page.getByRole("radio", { name: /High/ }));
    await user.type(page.getByRole("textbox", { name: "Jira" }), "DOC-12");
    await user.click(page.getByRole("button", { name: "Tomorrow" }));
    await user.click(page.getByRole("button", { name: /Create task/ }));

    expect(b.callsTo("add_task")).toEqual([
      { task: { title: "Write release notes", priority: "high", jira: "DOC-12", due: "tomorrow" } },
    ]);
    expect(await screen.findByTestId("row-6")).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("main", { name: "New task" })).toBeNull();
  });

  it("opens the new task page with ⌘N, creates with ⌘Enter, and leaves with Esc", async () => {
    const b = fakeBackend();
    const user = await boot();
    await user.keyboard("{Meta>}n{/Meta}");
    const page = within(screen.getByRole("main", { name: "New task" }));
    await user.type(page.getByRole("textbox", { name: "Title" }), "Quick one");
    await user.keyboard("{Meta>}{Enter}{/Meta}");
    expect(b.callsTo("add_task")).toEqual([{ task: { title: "Quick one", priority: "normal" } }]);
    await waitFor(() => expect(screen.queryByRole("main", { name: "New task" })).toBeNull());

    await user.keyboard("c");
    expect(screen.getByRole("main", { name: "New task" })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("main", { name: "New task" })).toBeNull();
    expect(b.callsTo("add_task")).toHaveLength(1);
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

describe("outside the app window", () => {
  it("explains that the page must be opened in the Tars app", async () => {
    const { OutsideApp } = await import("./components/Dialogs");
    render(<OutsideApp />);
    expect(screen.getByRole("alert")).toHaveTextContent("Open this in the Tars app");
    expect(screen.getByRole("alert")).toHaveTextContent("npx tauri dev");
  });
});

describe("layout", () => {
  it("collapses the sidebar with ⌘B and the preview with ⌘⇧B, the close button or Esc", async () => {
    fakeBackend();
    const user = await boot();
    await detail().findByText("Backoff, 5 tries");

    await user.keyboard("{Meta>}b{/Meta}");
    expect(screen.queryByRole("navigation", { name: "Views" })).toBeNull();
    await user.keyboard("{Meta>}b{/Meta}");
    expect(screen.getByRole("navigation", { name: "Views" })).toBeInTheDocument();

    await user.keyboard("{Meta>}{Shift>}b{/Shift}{/Meta}");
    expect(screen.queryByRole("region", { name: "Task details" })).toBeNull();
    await user.keyboard("{Meta>}{Shift>}b{/Shift}{/Meta}");
    expect(screen.getByRole("region", { name: "Task details" })).toBeInTheDocument();

    await user.click(detail().getByRole("button", { name: "Close preview" }));
    expect(screen.queryByRole("region", { name: "Task details" })).toBeNull();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("region", { name: "Task details" })).toBeInTheDocument();

    // Esc steps back: first out of the detail, then closes the preview.
    await user.keyboard("{Escape}{Escape}");
    expect(screen.queryByRole("region", { name: "Task details" })).toBeNull();

    // The choices survive a restart.
    expect(localStorage.getItem("tars.detailOpen")).toBe("0");
  });

  it("resizes the preview from the keyboard, remembers the width, and never squeezes the list", async () => {
    const original = window.innerWidth;
    Object.defineProperty(window, "innerWidth", { configurable: true, value: 1600 });
    try {
      fakeBackend();
      const user = await boot();
      const handle = await screen.findByRole("separator", { name: "Resize preview" });
      const before = Number(handle.getAttribute("aria-valuenow"));
      handle.focus();
      await user.keyboard("{ArrowLeft}{ArrowLeft}");
      const after = Number(screen.getByRole("separator").getAttribute("aria-valuenow"));
      expect(after).toBe(before + 48);
      expect(localStorage.getItem("tars.detailWidth")).toBe(String(after));

      // In a narrow window the preview gives way so the list keeps its minimum width.
      Object.defineProperty(window, "innerWidth", { configurable: true, value: 1000 });
      act(() => void window.dispatchEvent(new Event("resize")));
      expect(Number(screen.getByRole("separator").getAttribute("aria-valuenow"))).toBe(1000 - 208 - 360);
    } finally {
      Object.defineProperty(window, "innerWidth", { configurable: true, value: original });
    }
  });

  it("shows the legend with ? and returns with Esc", async () => {
    fakeBackend();
    const user = await boot();
    await user.keyboard("?");
    const legend = within(screen.getByRole("main", { name: "Legend" }));
    expect(legend.getByText(/Needs you\./)).toBeInTheDocument();
    expect(legend.getByText(/Agent update\./)).toBeInTheDocument();
    expect(legend.getByText(/In review\./)).toBeInTheDocument();
    expect(legend.getByText(/Urgent\./)).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("main", { name: "Legend" })).toBeNull();
    expect(screen.getByRole("navigation", { name: "Views" })).toBeInTheDocument();
  });
});

describe("editing", () => {
  it("edits a task with e and sends only what changed", async () => {
    const b = fakeBackend();
    const user = await boot();
    await detail().findByText("Backoff, 5 tries");
    await user.keyboard("e");
    const page = within(screen.getByRole("main", { name: "Edit #1" }));
    const title = page.getByRole("textbox", { name: "Title" });
    expect(title).toHaveValue("Add retry to webhook sender");
    await user.clear(title);
    await user.type(title, "Retry webhooks");
    await user.clear(page.getByPlaceholderText("BILL-881"));
    const tags = page.getByPlaceholderText("perf, backend");
    await user.clear(tags);
    await user.type(tags, "api");
    await user.keyboard("{Meta>}{Enter}{/Meta}");
    expect(b.callsTo("edit_task")).toEqual([{ id: 1, edit: { title: "Retry webhooks", jira: "", tag: ["api"], untag: ["backend"] } }]);
    await waitFor(() => expect(screen.getByTestId("row-1")).toHaveTextContent("Retry webhooks"));
    expect(screen.queryByRole("main", { name: "Edit #1" })).toBeNull();
  });

  it("opens the editor from the detail pane and saves nothing when nothing changed", async () => {
    const b = fakeBackend();
    const user = await boot();
    await detail().findByText("Backoff, 5 tries");
    await user.click(detail().getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: /Save changes/ }));
    expect(screen.queryByRole("main", { name: "Edit #1" })).toBeNull();
    expect(b.callsTo("edit_task")).toEqual([]);
  });

  it("changes priority from the picker and undoes it with u", async () => {
    const b = fakeBackend();
    const user = await boot();
    await detail().findByText("Backoff, 5 tries");
    await user.click(detail().getByRole("button", { name: "Priority: High" }));
    await user.click(within(screen.getByRole("listbox", { name: "Priority" })).getByRole("option", { name: "Urgent" }));
    expect(b.callsTo("edit_task")).toEqual([{ id: 1, edit: { priority: "urgent" } }]);
    await waitFor(() => expect(detail().getByRole("button", { name: "Priority: Urgent" })).toBeInTheDocument());
    await user.keyboard("u");
    await waitFor(() => expect(b.callsTo("edit_task")[1]).toEqual({ id: 1, edit: { priority: "high" } }));
    await waitFor(() => expect(detail().getByRole("button", { name: "Priority: High" })).toBeInTheDocument());
  });
});

describe("command palette", () => {
  it("finds a task in another view with ⌘K and opens it", async () => {
    fakeBackend();
    const user = await boot();
    await user.keyboard("{Meta>}k{/Meta}");
    const palette = within(screen.getByRole("dialog", { name: "Command palette" }));
    await user.keyboard("old clean");
    expect(palette.getAllByRole("option")[0]).toHaveTextContent("Old cleanup");
    await user.keyboard("{Enter}");
    expect(screen.queryByRole("dialog", { name: "Command palette" })).toBeNull();
    expect(heading()).toBe("Done");
    expect(screen.getByTestId("row-5")).toHaveAttribute("aria-selected", "true");
    await waitFor(() => expect(detail().getByText("Old cleanup")).toBeInTheDocument());
  });

  it("runs commands and closes with Esc", async () => {
    fakeBackend();
    const user = await boot();
    await user.keyboard("{Meta>}k{/Meta}legend{Enter}");
    expect(screen.getByRole("main", { name: "Legend" })).toBeInTheDocument();
    await user.keyboard("{Meta>}k{/Meta}");
    // Typing in the box doesn't fire the app's single-key shortcuts.
    await user.keyboard("x{Escape}");
    expect(screen.queryByRole("dialog", { name: "Command palette" })).toBeNull();
    expect(screen.getByRole("main", { name: "Legend" })).toBeInTheDocument();
  });
});

describe("grouping, sorting and filtering", () => {
  it("groups by status from the display menu, cycles with g, and remembers it", async () => {
    fakeBackend();
    const user = await boot();
    await user.keyboard("2");
    await user.click(screen.getByRole("button", { name: "Display options" }));
    await user.click(screen.getByRole("radio", { name: "Status" }));
    const groups = () => screen.getAllByRole("listbox").map((l) => l.getAttribute("aria-label"));
    expect(groups()).toEqual(["In review", "In progress", "Ready", "Blocked"]);
    expect(localStorage.getItem("tars.groupBy")).toBe("status");
    await user.click(document.body);
    await user.keyboard("g");
    expect(groups()).toEqual(["billing-svc", "No repo"]);
    await user.keyboard("g");
    expect(groups()).toEqual(["Tasks"]);
  });

  it("sorts by title", async () => {
    fakeBackend();
    const user = await boot();
    await user.keyboard("2");
    await user.click(screen.getByRole("button", { name: "Display options" }));
    await user.click(screen.getByRole("radio", { name: "No grouping" }));
    await user.click(screen.getByRole("radio", { name: "Title" }));
    const ids = screen.getAllByRole("option").map((o) => o.getAttribute("data-testid"));
    expect(ids).toEqual(["row-1", "row-2", "row-4", "row-3"]);
  });

  it("filters the list with / and clears with Esc", async () => {
    fakeBackend();
    const user = await boot();
    await user.keyboard("2/csv");
    expect(screen.getByRole("textbox", { name: "Filter tasks" })).toHaveFocus();
    expect(screen.getByTestId("row-4")).toBeInTheDocument();
    expect(screen.queryByTestId("row-1")).toBeNull();
    await user.keyboard("{Escape}");
    expect(screen.getByTestId("row-1")).toBeInTheDocument();
  });

  it("keeps Tab for focus while typing, so a draft survives", async () => {
    fakeBackend();
    const user = await boot();
    await detail().findByText("Backoff, 5 tries");
    await user.keyboard("n");
    const box = await detail().findByRole("textbox");
    await user.type(box, "half a thought");
    await user.keyboard("{Tab}");
    expect(heading()).toBe("Needs you");
    expect(detail().getByDisplayValue("half a thought")).toBeInTheDocument();
  });

  it("cycles views with Tab", async () => {
    fakeBackend();
    const user = await boot();
    await user.keyboard("{Tab}");
    expect(heading()).toBe("Open");
    await user.keyboard("{Shift>}{Tab}{/Shift}{Shift>}{Tab}{/Shift}");
    expect(heading()).toBe("Done");
  });
});

describe("panes and zoom", () => {
  it("opens the preview with a single click and toggles panes from header buttons", async () => {
    fakeBackend();
    const user = await boot();
    await user.click(screen.getByRole("button", { name: "Hide preview" }));
    expect(screen.queryByRole("region", { name: "Task details" })).toBeNull();
    await user.click(screen.getByTestId("row-1"));
    expect(screen.getByRole("region", { name: "Task details" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Hide sidebar" }));
    expect(screen.queryByRole("navigation", { name: "Views" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Show sidebar" }));
    expect(screen.getByRole("navigation", { name: "Views" })).toBeInTheDocument();
  });

  it("zooms with ⌘+ and ⌘−, resets with ⌘0, and remembers it", async () => {
    const b = fakeBackend();
    const user = await boot();
    await user.keyboard("{Meta>}={/Meta}{Meta>}={/Meta}");
    expect(b.callsTo("set_zoom").at(-1)).toEqual({ scale: 1.25 });
    expect(localStorage.getItem("tars.zoom")).toBe("1.25");
    await user.keyboard("{Meta>}-{/Meta}");
    expect(b.callsTo("set_zoom").at(-1)).toEqual({ scale: 1.1 });
    await user.keyboard("{Meta>}0{/Meta}");
    expect(b.callsTo("set_zoom").at(-1)).toEqual({ scale: 1 });
  });
});

describe("repos", () => {
  it("lists repos, adds one from the folder picker, and removes one after confirming", async () => {
    const b = fakeBackend();
    const user = await boot();
    await user.click(sidebar().getByRole("button", { name: "Settings" }));
    await user.click(screen.getByRole("button", { name: "Repos" }));
    const list = within(screen.getByRole("list", { name: "Your repos" }));
    expect(list.getByText("/Users/me/code/billing-svc")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Choose…" }));
    expect(screen.getByPlaceholderText("~/code/billing-svc")).toHaveValue("/Users/me/code/promos-ui");
    expect(screen.getByPlaceholderText("billing-svc")).toHaveValue("promos-ui");
    await user.click(screen.getByRole("button", { name: "Add repo" }));
    expect(b.callsTo("set_repo")).toEqual([{ name: "promos-ui", path: "/Users/me/code/promos-ui" }]);
    expect(await screen.findByText("/Users/me/code/promos-ui")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Remove billing-svc" }));
    expect(b.callsTo("remove_repo")).toEqual([]);
    await user.click(screen.getByRole("button", { name: "Delete billing-svc" }));
    expect(b.callsTo("remove_repo")).toEqual([{ name: "billing-svc" }]);
    await waitFor(() => expect(screen.queryByText("/Users/me/code/billing-svc")).toBeNull());
  });

  it("explains a relative path", async () => {
    fakeBackend();
    const user = await boot();
    await user.click(sidebar().getByRole("button", { name: "Settings" }));
    await user.click(screen.getByRole("button", { name: "Repos" }));
    await user.type(screen.getByPlaceholderText("~/code/billing-svc"), "code/x");
    await user.type(screen.getByPlaceholderText("billing-svc"), "x");
    await user.click(screen.getByRole("button", { name: "Add repo" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("full path");
  });
});

describe("right-click menu", () => {
  it("edits from the menu", async () => {
    fakeBackend();
    const user = await boot();
    await user.keyboard("2");
    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("row-3") });
    const menu = within(screen.getByRole("menu", { name: "#3 actions" }));
    await user.click(menu.getByRole("menuitem", { name: /Edit/ }));
    expect(screen.getByRole("main", { name: "Edit #3" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Title" })).toHaveValue("Tidy README");
  });

  it("deletes only after confirming, and Cancel keeps the task", async () => {
    const b = fakeBackend();
    const user = await boot();
    await user.keyboard("2");
    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("row-3") });
    await user.click(screen.getByRole("menuitem", { name: /Delete/ }));
    const dialog = screen.getByRole("alertdialog", { name: "Delete #3?" });
    expect(dialog).toHaveTextContent("Tidy README");
    // Focus starts on Cancel, so Enter keeps the task.
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(b.callsTo("delete_task")).toEqual([]);

    // ⌘⌫ asks for the selected task; confirming deletes it.
    await user.keyboard("{Meta>}{Backspace}{/Meta}");
    await user.click(within(screen.getByRole("alertdialog", { name: "Delete #3?" })).getByRole("button", { name: "Delete task" }));
    expect(b.callsTo("delete_task")).toEqual([{ id: 3 }]);
    await waitFor(() => expect(screen.queryByTestId("row-3")).toBeNull());
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });

  it("closes with Esc and leaves ⌘⌫ alone in text fields", async () => {
    const b = fakeBackend();
    const user = await boot();
    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("row-1") });
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).toBeNull();
    await user.keyboard("/ab{Meta>}{Backspace}{/Meta}");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(b.callsTo("delete_task")).toEqual([]);
  });
});
