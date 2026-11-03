import { useCallback, useEffect, useRef, useState, type DragEvent, type KeyboardEvent } from "react";
import { api, type TodoItem, type TodoList } from "../api";
import { ConfirmDialog } from "./Dialogs";
import { Page } from "./Pages";

// Same limits as internal/store/todos.go, checked here so mistakes show as
// you type instead of after a round trip.
export const MAX_LIST_NAME = 80;
export const MAX_TODO_TEXT = 500;
export const TODO_REFRESH_MS = 5000;

const hasControl = (s: string) => /[\u0000-\u001f\u007f]/.test(s);

/** Why a list name can't be used, or null when it can. */
export function listNameError(raw: string, lists: TodoList[], self?: number): string | null {
  const name = raw.trim();
  if (!name) return "A list needs a name.";
  if ([...name].length > MAX_LIST_NAME) return `Keep it under ${MAX_LIST_NAME} characters.`;
  if (hasControl(name)) return "A list name can't contain line breaks.";
  if (name.startsWith("-")) return 'A list name can\'t start with "-".';
  if (/^#?\d+$/.test(name)) return "A list name can't be just a number.";
  const lower = name.toLowerCase();
  if (lists.some((l) => l.id !== self && l.name.toLowerCase() === lower)) return `There's already a list called ${name}.`;
  return null;
}

/** Why an item's text can't be used, or null when it can. */
export function todoTextError(raw: string): string | null {
  const text = raw.trim();
  if (!text) return "A todo needs some text.";
  if ([...text].length > MAX_TODO_TEXT) return `Keep it under ${MAX_TODO_TEXT} characters.`;
  if (hasControl(text)) return "A todo can't contain line breaks.";
  return null;
}

const move = <T,>(xs: T[], from: number, to: number) => {
  const out = [...xs];
  const [x] = out.splice(from, 1);
  out.splice(to, 0, x);
  return out;
};

const renumber = (ls: TodoList[]) =>
  ls.map((l, i) => ({ ...l, position: i, items: l.items.map((it, j) => ({ ...it, position: j, list_id: l.id })) }));

// Lists as the user sees them. Every change shows at once, then goes to the
// CLI in order, one at a time, so two quick changes can't land out of order.
// Once nothing is waiting, the CLI's view replaces the local one, which also
// undoes any change it refused. New lists and items get a negative id until
// the CLI hands back the real one; queued changes look it up when they run.
function useTodos(onError: (e: unknown) => void, refreshMs: number) {
  const [lists, setLists] = useState<TodoList[] | null>(null);
  const current = useRef(lists);
  current.current = lists;
  const pending = useRef(0);
  const chain = useRef<Promise<void>>(Promise.resolve());
  const ids = useRef(new Map<number, number>());
  // Real id → the temporary one it replaced, so React keys stay put and a
  // field being typed in doesn't reset when the real id arrives.
  const temps = useRef(new Map<number, number>());
  const nextTemp = useRef(-1);
  const generation = useRef(0);
  const errorRef = useRef(onError);
  errorRef.current = onError;

  const load = useCallback(async () => {
    const gen = ++generation.current;
    try {
      const ls = await api.todos();
      // A change made while loading wins; the load after it brings the truth.
      if (gen === generation.current && pending.current === 0) setLists(ls);
    } catch (e) {
      errorRef.current(e);
    }
  }, []);

  useEffect(() => {
    void load();
    const h = setInterval(() => pending.current === 0 && void load(), refreshMs);
    return () => clearInterval(h);
  }, [load, refreshMs]);

  const real = (id: number) => (id > 0 ? id : ids.current.get(id));
  const learn = (temp: number, id: number) => {
    ids.current.set(temp, id);
    temps.current.set(id, temp);
  };
  const keyOf = (id: number) => temps.current.get(id) ?? id;
  const temp = () => nextTemp.current--;

  /** Shows local now and queues remote. remote gets real() to resolve ids. */
  const change = (local: (ls: TodoList[]) => TodoList[], remote: (r: typeof real) => Promise<unknown> | undefined) => {
    setLists((ls) => (ls ? renumber(local(ls)) : ls));
    pending.current++;
    generation.current++;
    chain.current = chain.current
      .then(() => remote(real))
      .then(
        () => {},
        (e) => errorRef.current(e),
      )
      .finally(() => {
        if (--pending.current === 0) void load();
      });
  };

  const actions = {
    addList(name: string) {
      const id = temp();
      const at = new Date().toISOString();
      change(
        (ls) => [...ls, { id, name: name.trim(), position: ls.length, created_at: at, updated_at: at, items: [] }],
        () => api.todoNewList(name).then((l) => learn(id, l.id)),
      );
    },
    renameList(id: number, name: string) {
      change(
        (ls) => ls.map((l) => (l.id === id ? { ...l, name: name.trim() } : l)),
        (r) => (r(id) ? api.todoRenameList(r(id)!, name) : undefined),
      );
    },
    moveList(id: number, to: number) {
      change(
        (ls) => move(ls, ls.findIndex((l) => l.id === id), to),
        (r) => (r(id) ? api.todoMoveList(r(id)!, to + 1) : undefined),
      );
    },
    removeList(id: number) {
      change(
        (ls) => ls.filter((l) => l.id !== id),
        (r) => (r(id) ? api.todoRemoveList(r(id)!, true) : undefined),
      );
    },
    addItem(list: number, text: string) {
      const id = temp();
      const at = new Date().toISOString();
      change(
        (ls) =>
          ls.map((l) =>
            l.id === list
              ? {
                  ...l,
                  items: [
                    ...l.items,
                    { id, list_id: list, text: text.trim(), done: false, position: l.items.length, created_at: at, updated_at: at },
                  ],
                }
              : l,
          ),
        (r) => (r(list) ? api.todoAdd(r(list)!, text).then((it) => learn(id, it.id)) : undefined),
      );
    },
    editItem(id: number, text: string) {
      change(
        (ls) => ls.map((l) => ({ ...l, items: l.items.map((it) => (it.id === id ? { ...it, text: text.trim() } : it)) })),
        (r) => (r(id) ? api.todoEdit(r(id)!, text) : undefined),
      );
    },
    toggle(id: number, done: boolean) {
      change(
        (ls) => ls.map((l) => ({ ...l, items: l.items.map((it) => (it.id === id ? { ...it, done } : it)) })),
        (r) => (r(id) ? api.todoCheck(r(id)!, done) : undefined),
      );
    },
    /** Puts an item at index `to` of list `into` (its own list or another). */
    moveItem(id: number, into: number, to: number) {
      const from = current.current?.find((l) => l.items.some((it) => it.id === id))?.id;
      if (from === undefined) return;
      change(
        (ls) => {
          const src = ls.find((l) => l.items.some((it) => it.id === id));
          if (!src) return ls;
          const item = src.items.find((it) => it.id === id)!;
          if (into === src.id) {
            return ls.map((l) => (l.id === into ? { ...l, items: move(l.items, l.items.indexOf(item), to) } : l));
          }
          return ls.map((l) => {
            if (l.id === src.id) return { ...l, items: l.items.filter((it) => it !== item) };
            if (l.id === into) {
              const items = [...l.items];
              items.splice(to, 0, item);
              return { ...l, items };
            }
            return l;
          });
        },
        (r) => {
          if (!r(id) || !r(into)) return undefined;
          return api.todoMove(r(id)!, to + 1, into === from ? null : r(into)!);
        },
      );
    },
    removeItem(id: number) {
      change(
        (ls) => ls.map((l) => ({ ...l, items: l.items.filter((it) => it.id !== id) })),
        (r) => (r(id) ? api.todoRemove(r(id)!) : undefined),
      );
    },
    clearDone(list: number) {
      change(
        (ls) => ls.map((l) => (l.id === list ? { ...l, items: l.items.filter((it) => !it.done) } : l)),
        (r) => (r(list) ? api.todoClear(r(list)!) : undefined),
      );
    },
  };
  return { lists, keyOf, ...actions };
}

type Todos = ReturnType<typeof useTodos>;

// Drag and drop: todos within and between lists, and lists among lists. The
// buttons and ⌥↑/⌥↓ do the same from the keyboard.
type Drag = { kind: "item"; id: number } | { kind: "list"; id: number };
type Over = { kind: "item"; id: number; after: boolean } | { kind: "list"; id: number } | null;
interface Dnd {
  drag: Drag | null;
  over: Over;
  start: (d: Drag) => void;
  hover: (o: Over) => void;
  end: () => void;
}

/** Where a dragged todo lands, as an index in the list after the move.
 *  Dropping before or after the item at `target`; `from` is its index now
 *  when it's already in this list. */
export function itemDropIndex(from: number | null, target: number, after: boolean) {
  const to = target + (after ? 1 : 0);
  return from !== null && from < to ? to - 1 : to;
}

// Is the pointer in the lower half of the element?
const lowerHalf = (e: DragEvent) => {
  const r = e.currentTarget.getBoundingClientRect();
  return e.clientY > r.top + r.height / 2;
};

const startDrag = (e: DragEvent, label: string) => {
  e.stopPropagation();
  e.dataTransfer?.setData("text/plain", label);
  if (e.dataTransfer) e.dataTransfer.effectAllowed = "move";
};

// Escape inside a field closes that field, not the page.
const stopEscape = (e: KeyboardEvent, then: () => void) => {
  if (e.key === "Escape") {
    e.stopPropagation();
    then();
  }
};

export function TodoPage({
  onError,
  onBack,
  refreshMs = TODO_REFRESH_MS,
}: {
  onError: (e: unknown) => void;
  onBack: () => void;
  refreshMs?: number;
}) {
  const todos = useTodos(onError, refreshMs);
  const [confirm, setConfirm] = useState<TodoList | null>(null);
  const [drag, setDrag] = useState<Drag | null>(null);
  const [over, setOver] = useState<Over>(null);
  const lists = todos.lists;
  const dnd: Dnd = {
    drag,
    over,
    start: (d) => (setDrag(d), setOver(null)),
    hover: (o) =>
      setOver((cur) =>
        cur?.kind === o?.kind && cur?.id === o?.id && (cur as { after?: boolean })?.after === (o as { after?: boolean })?.after
          ? cur
          : o,
      ),
    end: () => (setDrag(null), setOver(null)),
  };

  return (
    <Page title="Todo lists" onBack={onBack} full>
      <p className="page-lede">Plain checklists, kept apart from tasks: release steps, tech debt, review follow-ups.</p>
      <NewListForm lists={lists ?? []} onAdd={todos.addList} />
      {lists === null ? (
        <p className="muted">Loading…</p>
      ) : lists.length === 0 ? (
        <div className="todo-empty">
          <p className="todo-empty-title">No lists yet</p>
          <p className="muted">
            Name one above, or from a terminal: <span className="mono">tars todo list new tech-debt</span>
          </p>
        </div>
      ) : (
        <div className="todo-grid">
          {lists.map((l, i) => (
            <ListCard
              key={todos.keyOf(l.id)}
              list={l}
              index={i}
              lists={lists}
              todos={todos}
              dnd={dnd}
              onRemove={() => (l.items.length > 0 ? setConfirm(l) : todos.removeList(l.id))}
            />
          ))}
        </div>
      )}
      {confirm && (
        <ConfirmDialog
          title={`Delete ${confirm.name}?`}
          body={`It still has ${confirm.items.length} ${confirm.items.length === 1 ? "todo" : "todos"}, which will be deleted too. This can't be undone.`}
          confirmLabel="Delete list"
          onConfirm={async () => {
            todos.removeList(confirm.id);
            setConfirm(null);
          }}
          onCancel={() => setConfirm(null)}
        />
      )}
    </Page>
  );
}

function NewListForm({ lists, onAdd }: { lists: TodoList[]; onAdd: (name: string) => void }) {
  const [name, setName] = useState("");
  const [touched, setTouched] = useState(false);
  const error = listNameError(name, lists);
  return (
    <form
      className="todo-new-list"
      onSubmit={(e) => {
        e.preventDefault();
        setTouched(true);
        if (error) return;
        onAdd(name);
        setName("");
        setTouched(false);
      }}
    >
      <input
        value={name}
        maxLength={MAX_LIST_NAME + 20}
        placeholder="New list name"
        aria-label="New list name"
        aria-invalid={touched && !!error}
        onChange={(e) => {
          setName(e.target.value);
          setTouched(e.target.value !== "");
        }}
        onKeyDown={(e) => stopEscape(e, () => (setName(""), setTouched(false)))}
      />
      <button type="submit" className="primary" disabled={!!error}>
        Add list
      </button>
      {touched && error && (
        <p className="field-error" role="alert">
          {error}
        </p>
      )}
    </form>
  );
}

function ListCard({
  list,
  index,
  lists,
  todos,
  dnd,
  onRemove,
}: {
  list: TodoList;
  index: number;
  lists: TodoList[];
  todos: Todos;
  dnd: Dnd;
  onRemove: () => void;
}) {
  const [renaming, setRenaming] = useState(false);
  const [draft, setDraft] = useState(list.name);
  const done = list.items.filter((it) => it.done).length;
  const error = renaming ? listNameError(draft, lists, list.id) : null;

  const saveName = () => {
    if (error) return;
    if (draft.trim() !== list.name) todos.renameList(list.id, draft);
    setRenaming(false);
  };

  const d = dnd.drag;
  // A todo dropped on the card, not on another todo, goes to the end.
  const onDragOver = (e: DragEvent) => {
    if (!d || (d.kind === "list" && d.id === list.id)) return;
    e.preventDefault();
    dnd.hover({ kind: "list", id: list.id });
  };
  const onDrop = (e: DragEvent) => {
    if (!d) return;
    e.preventDefault();
    dnd.end();
    if (d.kind === "list") {
      if (d.id !== list.id) todos.moveList(d.id, index);
      return;
    }
    const here = list.items.some((it) => it.id === d.id);
    todos.moveItem(d.id, list.id, here ? list.items.length - 1 : list.items.length);
  };
  const classes = ["todo-card"];
  if (d?.kind === "list" && d.id === list.id) classes.push("dragging");
  if (dnd.over?.kind === "list" && dnd.over.id === list.id) classes.push("drop-target");

  return (
    <section className={classes.join(" ")} aria-label={list.name} onDragOver={onDragOver} onDrop={onDrop}>
      <header
        className="todo-card-head"
        draggable={!renaming}
        data-testid={`drag-list-${list.name}`}
        onDragStart={(e) => (startDrag(e, list.name), dnd.start({ kind: "list", id: list.id }))}
        onDragEnd={dnd.end}
      >
        {renaming ? (
          <form
            className="todo-rename"
            onSubmit={(e) => {
              e.preventDefault();
              saveName();
            }}
          >
            <input
              autoFocus
              value={draft}
              aria-label={`Rename ${list.name}`}
              aria-invalid={!!error}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => stopEscape(e, () => setRenaming(false))}
            />
            <button type="submit" className="primary" disabled={!!error}>
              Save
            </button>
            {error && (
              <p className="field-error" role="alert">
                {error}
              </p>
            )}
          </form>
        ) : (
          <>
            <h2 className="todo-card-title" onDoubleClick={() => (setDraft(list.name), setRenaming(true))}>
              {list.name}
            </h2>
            <span className="todo-count" data-testid={`todo-count-${list.name}`}>
              {done}/{list.items.length}
            </span>
          </>
        )}
        {!renaming && (
          <div className="todo-card-actions">
            <IconButton label={`Move ${list.name} up`} disabled={index === 0} onClick={() => todos.moveList(list.id, index - 1)}>
              ↑
            </IconButton>
            <IconButton
              label={`Move ${list.name} down`}
              disabled={index === lists.length - 1}
              onClick={() => todos.moveList(list.id, index + 1)}
            >
              ↓
            </IconButton>
            <IconButton label={`Rename ${list.name}`} onClick={() => (setDraft(list.name), setRenaming(true))}>
              ✎
            </IconButton>
            <IconButton label={`Delete ${list.name}`} onClick={onRemove} danger>
              ×
            </IconButton>
          </div>
        )}
      </header>
      {list.items.length === 0 ? (
        <p className="todo-none muted">Nothing here yet.</p>
      ) : (
        <ul className="todo-items" aria-label={`${list.name} todos`}>
          {list.items.map((it, i) => (
            <ItemRow key={todos.keyOf(it.id)} item={it} index={i} list={list} lists={lists} todos={todos} dnd={dnd} />
          ))}
        </ul>
      )}
      <AddItemForm list={list} onAdd={(text) => todos.addItem(list.id, text)} />
      {done > 0 && (
        <button className="link todo-clear" onClick={() => todos.clearDone(list.id)}>
          Clear {done} checked
        </button>
      )}
    </section>
  );
}

function ItemRow({
  item,
  index,
  list,
  lists,
  todos,
  dnd,
}: {
  item: TodoItem;
  index: number;
  list: TodoList;
  lists: TodoList[];
  todos: Todos;
  dnd: Dnd;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(item.text);
  const error = editing ? todoTextError(draft) : null;
  const others = lists.filter((l) => l.id !== list.id);
  const last = list.items.length - 1;

  const save = () => {
    if (error) return;
    if (draft.trim() !== item.text) todos.editItem(item.id, draft);
    setEditing(false);
  };

  // ⌥↑ / ⌥↓ reorder the focused item, like most list apps.
  const onKeyDown = (e: KeyboardEvent) => {
    if (!e.altKey || editing) return;
    if (e.key === "ArrowUp" && index > 0) todos.moveItem(item.id, list.id, index - 1);
    else if (e.key === "ArrowDown" && index < last) todos.moveItem(item.id, list.id, index + 1);
    else return;
    e.preventDefault();
    e.stopPropagation();
  };

  const d = dnd.drag?.kind === "item" ? dnd.drag : null;
  const onDragOver = (e: DragEvent) => {
    if (!d) return;
    e.preventDefault();
    e.stopPropagation();
    dnd.hover({ kind: "item", id: item.id, after: lowerHalf(e) });
  };
  const onDrop = (e: DragEvent) => {
    if (!d) return;
    e.preventDefault();
    e.stopPropagation();
    dnd.end();
    if (d.id === item.id) return;
    const from = list.items.findIndex((it) => it.id === d.id);
    const to = itemDropIndex(from >= 0 ? from : null, index, lowerHalf(e));
    if (from !== to) todos.moveItem(d.id, list.id, to);
  };
  const classes = ["todo-item"];
  if (item.done) classes.push("done");
  if (d?.id === item.id) classes.push("dragging");
  const o = dnd.over;
  if (d && d.id !== item.id && o?.kind === "item" && o.id === item.id) classes.push(o.after ? "drop-after" : "drop-before");

  return (
    <li
      className={classes.join(" ")}
      data-testid={`todo-${item.text}`}
      onKeyDown={onKeyDown}
      draggable={!editing}
      onDragStart={(e) => (startDrag(e, item.text), dnd.start({ kind: "item", id: item.id }))}
      onDragOver={onDragOver}
      onDrop={onDrop}
      onDragEnd={dnd.end}
    >
      <input
        type="checkbox"
        checked={item.done}
        aria-label={item.text}
        onChange={(e) => todos.toggle(item.id, e.target.checked)}
      />
      {editing ? (
        <form
          className="todo-edit"
          onSubmit={(e) => {
            e.preventDefault();
            save();
          }}
        >
          <input
            autoFocus
            value={draft}
            aria-label={`Edit ${item.text}`}
            aria-invalid={!!error}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => stopEscape(e, () => setEditing(false))}
            onBlur={() => (error ? setEditing(false) : save())}
          />
          {error && (
            <p className="field-error" role="alert">
              {error}
            </p>
          )}
        </form>
      ) : (
        <span className="todo-text" onDoubleClick={() => (setDraft(item.text), setEditing(true))}>
          {item.text}
        </span>
      )}
      {!editing && (
        <div className="todo-item-actions">
          <IconButton label={`Move ${item.text} up`} disabled={index === 0} onClick={() => todos.moveItem(item.id, list.id, index - 1)}>
            ↑
          </IconButton>
          <IconButton
            label={`Move ${item.text} down`}
            disabled={index === last}
            onClick={() => todos.moveItem(item.id, list.id, index + 1)}
          >
            ↓
          </IconButton>
          {others.length > 0 && (
            <select
              className="todo-move-to"
              aria-label={`Move ${item.text} to list`}
              value=""
              onChange={(e) => {
                const to = lists.find((l) => l.id === Number(e.target.value));
                if (to) todos.moveItem(item.id, to.id, to.items.length);
              }}
            >
              <option value="" disabled>
                Move to…
              </option>
              {others.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.name}
                </option>
              ))}
            </select>
          )}
          <IconButton label={`Edit ${item.text}`} onClick={() => (setDraft(item.text), setEditing(true))}>
            ✎
          </IconButton>
          <IconButton label={`Delete ${item.text}`} onClick={() => todos.removeItem(item.id)} danger>
            ×
          </IconButton>
        </div>
      )}
    </li>
  );
}

function AddItemForm({ list, onAdd }: { list: TodoList; onAdd: (text: string) => void }) {
  const [text, setText] = useState("");
  const error = text ? todoTextError(text) : null;
  return (
    <form
      className="todo-add"
      onSubmit={(e) => {
        e.preventDefault();
        if (todoTextError(text)) return;
        onAdd(text);
        setText("");
      }}
    >
      <input
        value={text}
        placeholder="Add a todo"
        aria-label={`Add a todo to ${list.name}`}
        aria-invalid={!!error}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => stopEscape(e, () => setText(""))}
      />
      {error && (
        <p className="field-error" role="alert">
          {error}
        </p>
      )}
    </form>
  );
}

function IconButton({
  label,
  onClick,
  disabled,
  danger,
  children,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  danger?: boolean;
  children: string;
}) {
  return (
    <button
      type="button"
      className={"todo-icon" + (danger ? " danger-icon" : "")}
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
    >
      <span aria-hidden="true">{children}</span>
    </button>
  );
}
