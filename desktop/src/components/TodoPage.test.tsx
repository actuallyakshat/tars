import { createEvent, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "../App";
import { fakeBackend } from "../test/backend";
import { itemDropIndex, listNameError, todoTextError } from "./TodoPage";

async function openTodos() {
  const user = userEvent.setup();
  render(<App />);
  const nav = await screen.findByRole("navigation", { name: "Views" });
  await user.click(within(nav).getByRole("button", { name: /Todo lists/ }));
  await screen.findByRole("region", { name: "Release" });
  return user;
}

const card = (name: string) => within(screen.getByRole("region", { name }));
const items = (name: string) =>
  card(name)
    .queryAllByRole("listitem")
    .map((li) => li.textContent?.match(/^[^↑]*/)?.[0].trim());
const listOrder = () => screen.getAllByRole("region").map((r) => r.getAttribute("aria-label"));

describe("validation helpers", () => {
  it("checks list names the way the CLI does", () => {
    const lists = [{ id: 1, name: "Release", position: 0, created_at: "", updated_at: "", items: [] }];
    expect(listNameError("  ", lists)).toMatch(/needs a name/);
    expect(listNameError("42", lists)).toMatch(/just a number/);
    expect(listNameError("#7", lists)).toMatch(/just a number/);
    expect(listNameError("-x", lists)).toMatch(/start with/);
    expect(listNameError("a\nb", lists)).toMatch(/line breaks/);
    expect(listNameError("x".repeat(81), lists)).toMatch(/under 80/);
    expect(listNameError("RELEASE", lists)).toMatch(/already a list/);
    // Its own name, in another case, is fine when renaming.
    expect(listNameError("RELEASE", lists, 1)).toBeNull();
    expect(listNameError("Q4 launch", lists)).toBeNull();
  });

  it("works out where a dropped todo lands", () => {
    // In its own list: before or after a later item, then an earlier one.
    expect(itemDropIndex(0, 2, false)).toBe(1);
    expect(itemDropIndex(0, 2, true)).toBe(2);
    expect(itemDropIndex(2, 0, false)).toBe(0);
    expect(itemDropIndex(2, 0, true)).toBe(1);
    // From another list nothing shifts.
    expect(itemDropIndex(null, 0, false)).toBe(0);
    expect(itemDropIndex(null, 1, true)).toBe(2);
  });

  it("checks todo text", () => {
    expect(todoTextError(" ")).toMatch(/needs some text/);
    expect(todoTextError("x".repeat(501))).toMatch(/under 500/);
    expect(todoTextError("a\tb")).toMatch(/line breaks/);
    expect(todoTextError("-")).toBeNull();
  });
});

describe("todo lists", () => {
  it("opens from the sidebar, above Repos, and shows each list with its progress", async () => {
    fakeBackend();
    await openTodos();
    const foot = screen.getByRole("navigation", { name: "Views" }).querySelector(".sidebar-foot")!;
    const links = [...foot.querySelectorAll("button")].map((b) => b.textContent);
    expect(links.indexOf("Todo lists")).toBeGreaterThanOrEqual(0);
    expect(links.indexOf("Todo lists")).toBeLessThan(links.indexOf("Repos"));
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Todo lists");
    expect(listOrder()).toEqual(["Release", "Cleanup"]);
    expect(items("Release")).toEqual(["Update CHANGELOG", "Tag v0.4.0", "Publish the cask"]);
    expect(screen.getByTestId("todo-count-Release")).toHaveTextContent("1/3");
    expect(card("Release").getByRole("checkbox", { name: "Tag v0.4.0" })).toBeChecked();
  });

  it("shows changes before the CLI answers", async () => {
    const b = fakeBackend();
    const user = await openTodos();
    const release = b.holdTodos();

    await user.type(card("Release").getByRole("textbox", { name: "Add a todo to Release" }), "Announce in eng{Enter}");
    await user.click(card("Release").getByRole("checkbox", { name: "Update CHANGELOG" }));
    await user.click(card("Cleanup").getByRole("button", { name: "Move Cleanup up" }));

    // All on screen while every call is still waiting.
    expect(items("Release")).toEqual(["Update CHANGELOG", "Tag v0.4.0", "Publish the cask", "Announce in eng"]);
    expect(card("Release").getByRole("checkbox", { name: "Update CHANGELOG" })).toBeChecked();
    expect(listOrder()).toEqual(["Cleanup", "Release"]);
    expect(b.todos[1].items).toHaveLength(1);
    expect(b.todos[0].name).toBe("Release");

    release();
    await waitFor(() => expect(b.todos.map((l) => l.name)).toEqual(["Cleanup", "Release"]));
    expect(b.todos[1].items.map((i) => i.text)).toEqual(["Update CHANGELOG", "Tag v0.4.0", "Publish the cask", "Announce in eng"]);
    expect(b.todos[1].items[0].done).toBe(true);
    // The calls ran in the order they were made.
    const order = b.calls.map(([c]) => c).filter((c) => c.startsWith("todo_"));
    expect(order).toEqual(["todo_add", "todo_check", "todo_move_list"]);
  });

  it("edits a todo added a moment ago, before the CLI has given it an id", async () => {
    const b = fakeBackend();
    const user = await openTodos();
    const release = b.holdTodos();
    await user.type(card("Cleanup").getByRole("textbox", { name: "Add a todo to Cleanup" }), "Retry flaky{Enter}");
    await user.click(card("Cleanup").getByRole("button", { name: "Edit Retry flaky" }));
    const input = card("Cleanup").getByRole("textbox", { name: "Edit Retry flaky" });
    await user.clear(input);
    await user.type(input, "Retry the flaky e2e test{Enter}");
    expect(items("Cleanup")).toEqual(["Drop the legacy sync flag", "Retry the flaky e2e test"]);

    release();
    await waitFor(() => expect(b.todos[1].items.map((i) => i.text)).toEqual(["Drop the legacy sync flag", "Retry the flaky e2e test"]));
    expect(b.callsTo("todo_edit")).toEqual([{ id: 5, text: "Retry the flaky e2e test" }]);
  });

  it("keeps what you're typing in a new list when the CLI hands back its id", async () => {
    const b = fakeBackend();
    const user = await openTodos();
    await user.type(screen.getByRole("textbox", { name: "New list name" }), "Oncall{Enter}");
    const box = card("Oncall").getByRole("textbox", { name: "Add a todo to Oncall" });
    await user.type(box, "Rotate the pager");
    await waitFor(() => expect(b.todos.map((l) => l.name)).toContain("Oncall"));
    await waitFor(() => expect(b.callsTo("list_todos").length).toBeGreaterThan(1));
    expect(card("Oncall").getByRole("textbox", { name: "Add a todo to Oncall" })).toBe(box);
    expect(box).toHaveValue("Rotate the pager");
    expect(box).toHaveFocus();
  });

  it("puts the change back and says why when the CLI refuses it", async () => {
    const b = fakeBackend();
    const user = await openTodos();
    b.fail.todo_remove = { kind: "failed", message: "database is locked" };
    const release = b.holdTodos();
    await user.click(card("Release").getByRole("button", { name: "Delete Publish the cask" }));
    expect(items("Release")).toEqual(["Update CHANGELOG", "Tag v0.4.0"]);
    release();
    expect(await screen.findByText("database is locked")).toBeInTheDocument();
    await waitFor(() => expect(items("Release")).toEqual(["Update CHANGELOG", "Tag v0.4.0", "Publish the cask"]));
  });

  it("creates, renames and validates lists", async () => {
    const b = fakeBackend();
    const user = await openTodos();
    const name = screen.getByRole("textbox", { name: "New list name" });
    const add = screen.getByRole("button", { name: "Add list" });

    await user.type(name, "cleanup");
    expect(screen.getByRole("alert")).toHaveTextContent("There's already a list called cleanup.");
    expect(add).toBeDisabled();
    await user.clear(name);
    await user.type(name, "123");
    expect(screen.getByRole("alert")).toHaveTextContent("can't be just a number");
    await user.clear(name);
    await user.type(name, "  Oncall  {Enter}");
    expect(listOrder()).toEqual(["Release", "Cleanup", "Oncall"]);
    expect(name).toHaveValue("");
    await waitFor(() => expect(b.todos.map((l) => l.name)).toEqual(["Release", "Cleanup", "Oncall"]));

    await user.click(card("Oncall").getByRole("button", { name: "Rename Oncall" }));
    const rename = card("Oncall").getByRole("textbox", { name: "Rename Oncall" });
    await user.clear(rename);
    await user.type(rename, "Release");
    expect(card("Oncall").getByRole("alert")).toHaveTextContent("already a list");
    await user.clear(rename);
    await user.type(rename, "Oncall handoff{Enter}");
    expect(screen.getByRole("region", { name: "Oncall handoff" })).toBeInTheDocument();
    await waitFor(() => expect(b.todos[2].name).toBe("Oncall handoff"));
    expect(b.callsTo("todo_rename_list")).toEqual([{ id: 3, name: "Oncall handoff" }]);
  });

  it("won't add an empty todo, and Esc in a field leaves the page open", async () => {
    const b = fakeBackend();
    const user = await openTodos();
    const box = card("Cleanup").getByRole("textbox", { name: "Add a todo to Cleanup" });
    await user.type(box, "   {Enter}");
    expect(card("Cleanup").getByRole("alert")).toHaveTextContent("needs some text");
    expect(b.callsTo("todo_add")).toEqual([]);
    await user.keyboard("{Escape}");
    expect(box).toHaveValue("");
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Todo lists");
    // Esc outside a field goes back to the task list.
    await user.click(document.body);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("region", { name: "Cleanup" })).toBeNull();
  });

  it("reorders todos with buttons and ⌥↑/⌥↓, and moves one to another list", async () => {
    const b = fakeBackend();
    const user = await openTodos();
    expect(card("Release").getByRole("button", { name: "Move Update CHANGELOG up" })).toBeDisabled();
    expect(card("Release").getByRole("button", { name: "Move Publish the cask down" })).toBeDisabled();

    await user.click(card("Release").getByRole("button", { name: "Move Publish the cask up" }));
    expect(items("Release")).toEqual(["Update CHANGELOG", "Publish the cask", "Tag v0.4.0"]);

    card("Release").getByRole("checkbox", { name: "Update CHANGELOG" }).focus();
    await user.keyboard("{Alt>}{ArrowDown}{/Alt}");
    expect(items("Release")).toEqual(["Publish the cask", "Update CHANGELOG", "Tag v0.4.0"]);

    await user.selectOptions(card("Release").getByRole("combobox", { name: "Move Tag v0.4.0 to list" }), "Cleanup");
    expect(items("Release")).toEqual(["Publish the cask", "Update CHANGELOG"]);
    expect(items("Cleanup")).toEqual(["Drop the legacy sync flag", "Tag v0.4.0"]);

    await waitFor(() => expect(b.todos[1].items.map((i) => i.text)).toEqual(["Drop the legacy sync flag", "Tag v0.4.0"]));
    expect(b.todos[0].items.map((i) => i.text)).toEqual(["Publish the cask", "Update CHANGELOG"]);
    expect(b.callsTo("todo_move")).toEqual([
      { id: 3, position: 2, list: null },
      { id: 1, position: 2, list: null },
      { id: 2, position: 2, list: 2 },
    ]);
  });

  it("drags todos within a list, onto another list's todo, and onto another list's card", async () => {
    const b = fakeBackend();
    await openTodos();
    const row = (text: string) => screen.getByTestId(`todo-${text}`);
    // jsdom has no DragEvent, so the pointer position is set by hand.
    const at = (ev: Event, clientY: number) => (Object.defineProperty(ev, "clientY", { value: clientY }), ev);
    const drag = (from: HTMLElement, to: HTMLElement, clientY: number) => {
      fireEvent.dragStart(from);
      fireEvent(to, at(createEvent.dragOver(to), clientY));
      fireEvent(to, at(createEvent.drop(to), clientY));
      fireEvent.dragEnd(from);
    };

    // The bottom half of a todo drops after it.
    drag(row("Update CHANGELOG"), row("Publish the cask"), 10);
    expect(items("Release")).toEqual(["Tag v0.4.0", "Publish the cask", "Update CHANGELOG"]);
    // The top half drops before it, here in another list.
    drag(row("Tag v0.4.0"), row("Drop the legacy sync flag"), 0);
    expect(items("Cleanup")).toEqual(["Tag v0.4.0", "Drop the legacy sync flag"]);
    // Anywhere else on a card puts it at the end.
    drag(row("Publish the cask"), screen.getByRole("region", { name: "Cleanup" }), 0);
    expect(items("Cleanup")).toEqual(["Tag v0.4.0", "Drop the legacy sync flag", "Publish the cask"]);
    expect(items("Release")).toEqual(["Update CHANGELOG"]);

    await waitFor(() => expect(b.todos[1].items.map((i) => i.text)).toEqual(["Tag v0.4.0", "Drop the legacy sync flag", "Publish the cask"]));
    expect(b.callsTo("todo_move")).toEqual([
      { id: 1, position: 3, list: null },
      { id: 2, position: 1, list: 2 },
      { id: 3, position: 3, list: 2 },
    ]);
  });

  it("drags a list onto another to take its place, and dropping on itself does nothing", async () => {
    const b = fakeBackend();
    b.todos.push({ id: 3, name: "Oncall", position: 2, created_at: "", updated_at: "", items: [] });
    await openTodos();
    const handle = screen.getByTestId("drag-list-Oncall");
    fireEvent.dragStart(handle);
    fireEvent.dragOver(screen.getByRole("region", { name: "Release" }));
    expect(screen.getByRole("region", { name: "Release" })).toHaveClass("drop-target");
    fireEvent.drop(screen.getByRole("region", { name: "Release" }));
    expect(listOrder()).toEqual(["Oncall", "Release", "Cleanup"]);

    fireEvent.dragStart(screen.getByTestId("drag-list-Cleanup"));
    fireEvent.drop(screen.getByRole("region", { name: "Cleanup" }));
    expect(listOrder()).toEqual(["Oncall", "Release", "Cleanup"]);
    await waitFor(() => expect(b.todos.map((l) => l.name)).toEqual(["Oncall", "Release", "Cleanup"]));
    expect(b.callsTo("todo_move_list")).toEqual([{ id: 3, position: 1 }]);
  });

  it("checks, clears checked and deletes todos", async () => {
    const b = fakeBackend();
    const user = await openTodos();
    await user.click(card("Release").getByRole("checkbox", { name: "Tag v0.4.0" }));
    expect(screen.getByTestId("todo-count-Release")).toHaveTextContent("0/3");
    await user.click(card("Release").getByRole("checkbox", { name: "Publish the cask" }));
    await user.click(card("Release").getByRole("button", { name: "Clear 1 checked" }));
    expect(items("Release")).toEqual(["Update CHANGELOG", "Tag v0.4.0"]);
    await user.click(card("Release").getByRole("button", { name: "Delete Update CHANGELOG" }));
    expect(items("Release")).toEqual(["Tag v0.4.0"]);
    await waitFor(() => expect(b.todos[0].items.map((i) => i.text)).toEqual(["Tag v0.4.0"]));
    expect(b.todos[0].items[0].done).toBe(false);
  });

  it("deletes an empty list at once, and asks first when it still has todos", async () => {
    const b = fakeBackend();
    b.todos.push({ id: 3, name: "Empty", position: 2, created_at: "", updated_at: "", items: [] });
    const user = await openTodos();
    await user.click(card("Empty").getByRole("button", { name: "Delete Empty" }));
    expect(screen.queryByRole("region", { name: "Empty" })).toBeNull();
    expect(screen.queryByRole("alertdialog")).toBeNull();

    await user.click(card("Cleanup").getByRole("button", { name: "Delete Cleanup" }));
    const dialog = within(screen.getByRole("alertdialog"));
    expect(dialog.getByText(/still has 1 todo,/)).toBeInTheDocument();
    await user.click(dialog.getByRole("button", { name: "Cancel" }));
    expect(screen.getByRole("region", { name: "Cleanup" })).toBeInTheDocument();

    await user.click(card("Cleanup").getByRole("button", { name: "Delete Cleanup" }));
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Delete list" }));
    expect(screen.queryByRole("region", { name: "Cleanup" })).toBeNull();
    await waitFor(() => expect(b.todos.map((l) => l.name)).toEqual(["Release"]));
    expect(b.callsTo("todo_remove_list")).toEqual([{ id: 3, force: true }, { id: 2, force: true }]);
  });

  it("shows an empty state", async () => {
    const b = fakeBackend();
    b.todos = [];
    const user = userEvent.setup();
    render(<App />);
    await user.click(within(await screen.findByRole("navigation", { name: "Views" })).getByRole("button", { name: /Todo lists/ }));
    expect(await screen.findByText("No lists yet")).toBeInTheDocument();
  });
});
