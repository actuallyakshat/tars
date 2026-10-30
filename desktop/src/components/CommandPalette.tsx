import { useEffect, useId, useMemo, useRef, useState } from "react";
import type { Task } from "../api";
import { STATUS, matchesQuery } from "../model";

export interface Command {
  id: string;
  label: string;
  keys?: string;
  run: () => void;
}

type Item = { kind: "task"; task: Task } | { kind: "command"; command: Command };

const MAX_TASKS = 50;

// ⌘K: find any task by title, #id, Jira key, repo or tag, or run a command.
export function CommandPalette({
  tasks,
  commands,
  onTask,
  onClose,
}: {
  tasks: Task[];
  commands: Command[];
  onTask: (id: number) => void;
  onClose: () => void;
}) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const list = useRef<HTMLUListElement>(null);
  const listId = useId();

  useEffect(() => input.current?.focus(), []);

  const items = useMemo<Item[]>(() => {
    const words = query.toLowerCase().split(/\s+/).filter(Boolean);
    // Open work first, most recently touched first.
    const found = tasks
      .filter((t) => matchesQuery(t, query))
      .sort((a, b) => Number(a.status === "done") - Number(b.status === "done") || b.updated_at.localeCompare(a.updated_at))
      .slice(0, MAX_TASKS)
      .map((task): Item => ({ kind: "task", task }));
    const cmds = commands
      .filter((c) => words.every((w) => c.label.toLowerCase().includes(w)))
      .map((command): Item => ({ kind: "command", command }));
    return words.length ? [...found, ...cmds] : [...cmds, ...found];
  }, [query, tasks, commands]);

  useEffect(() => setActive(0), [query]);
  useEffect(() => {
    list.current?.querySelector(`[data-index="${active}"]`)?.scrollIntoView({ block: "nearest" });
  }, [active]);

  const choose = (item: Item | undefined) => {
    if (!item) return;
    onClose();
    if (item.kind === "task") onTask(item.task.id);
    else item.command.run();
  };

  const onKey = (e: React.KeyboardEvent) => {
    e.stopPropagation(); // the app's shortcuts stay out of the search box
    const n = items.length;
    const down = e.key === "ArrowDown" || (e.ctrlKey && e.key === "n");
    const up = e.key === "ArrowUp" || (e.ctrlKey && e.key === "p");
    if (down && n) setActive((i) => (i + 1) % n);
    else if (up && n) setActive((i) => (i - 1 + n) % n);
    else if (e.key === "Enter") choose(items[active]);
    else if (e.key === "Escape" || (e.metaKey && e.key.toLowerCase() === "k")) onClose();
    else return;
    e.preventDefault();
  };

  const optionId = (i: number) => `${listId}-${i}`;
  let lastKind: Item["kind"] | null = null;

  return (
    <div className="palette-backdrop" onMouseDown={onClose}>
      <div className="palette" role="dialog" aria-modal="true" aria-label="Command palette" onMouseDown={(e) => e.stopPropagation()}>
        <div className="palette-search">
          <svg width="15" height="15" viewBox="0 0 15 15" aria-hidden="true">
            <circle cx="6.5" cy="6.5" r="4.6" fill="none" stroke="currentColor" strokeWidth="1.4" />
            <path d="m10 10 3.4 3.4" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
          </svg>
          <input
            ref={input}
            role="combobox"
            aria-label="Search tasks and commands"
            aria-expanded="true"
            aria-controls={listId}
            aria-activedescendant={items.length ? optionId(active) : undefined}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onKey}
            placeholder="Search tasks by title, #id, Jira key, repo or tag…"
            spellCheck={false}
          />
          <kbd>esc</kbd>
        </div>
        <ul className="palette-list" id={listId} role="listbox" aria-label="Results" ref={list}>
          {items.length === 0 && <li className="palette-empty">Nothing matches “{query}”.</li>}
          {items.map((item, i) => {
            const heading = item.kind !== lastKind ? (item.kind === "task" ? "Tasks" : "Commands") : null;
            lastKind = item.kind;
            return (
              <PaletteRow
                key={item.kind === "task" ? `t${item.task.id}` : `c${item.command.id}`}
                id={optionId(i)}
                index={i}
                heading={heading}
                item={item}
                active={i === active}
                onHover={() => setActive(i)}
                onChoose={() => choose(item)}
              />
            );
          })}
        </ul>
      </div>
    </div>
  );
}

function PaletteRow({
  id,
  index,
  heading,
  item,
  active,
  onHover,
  onChoose,
}: {
  id: string;
  index: number;
  heading: string | null;
  item: Item;
  active: boolean;
  onHover: () => void;
  onChoose: () => void;
}) {
  return (
    <>
      {heading && (
        <li className="palette-heading" role="presentation">
          {heading}
        </li>
      )}
      <li
        id={id}
        data-index={index}
        role="option"
        aria-selected={active}
        className={"palette-item" + (active ? " active" : "")}
        onMouseMove={onHover}
        onClick={onChoose}
      >
        {item.kind === "task" ? (
          <>
            <span className={`palette-icon st-${item.task.status}`} aria-hidden="true">
              {STATUS[item.task.status].icon}
            </span>
            <span className={"palette-label" + (item.task.status === "done" ? " is-done" : "")}>{item.task.title}</span>
            <span className="palette-meta mono">
              {item.task.jira && <span>{item.task.jira}</span>}
              <span>#{item.task.id}</span>
            </span>
          </>
        ) : (
          <>
            <span className="palette-icon" aria-hidden="true">
              ›
            </span>
            <span className="palette-label">{item.command.label}</span>
            {item.command.keys && <kbd>{item.command.keys}</kbd>}
          </>
        )}
      </li>
    </>
  );
}
