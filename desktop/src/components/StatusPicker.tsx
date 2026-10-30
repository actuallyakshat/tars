import { useEffect, useId, useRef, useState } from "react";
import type { Status } from "../api";
import { PRIORITIES, STATUS, STATUSES, priorityMark } from "../model";

interface Option<T> {
  value: T;
  label: string;
  icon: string;
  className: string;
}

const STATUS_OPTIONS: Option<Status>[] = STATUSES.map((s) => ({ value: s, label: STATUS[s].label, icon: STATUS[s].icon, className: `st-${s}` }));

const PRIORITY_OPTIONS: Option<number>[] = PRIORITIES.map((name, i) => ({
  value: i,
  label: name,
  icon: priorityMark(i) || "–",
  className: `p${i}`,
}));

export function StatusPicker({ value, onChange }: { value: Status; onChange: (s: Status) => void }) {
  return <Picker id="status-select" name="Status" options={STATUS_OPTIONS} value={value} onChange={onChange} />;
}

export function PriorityPicker({ value, onChange }: { value: number; onChange: (p: number) => void }) {
  return <Picker id="priority-select" name="Priority" options={PRIORITY_OPTIONS} value={value} onChange={onChange} />;
}

// A button that opens a small menu of choices, each in its own colour.
// Keyboard: ↑↓ to move, Enter or Space to choose, Esc to close, or type a
// choice's first letter.
function Picker<T>({ id, name, options, value, onChange }: { id: string; name: string; options: Option<T>[]; value: T; onChange: (v: T) => void }) {
  const index = Math.max(0, options.findIndex((o) => o.value === value));
  const current = options[index];
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(index);
  const button = useRef<HTMLButtonElement>(null);
  const list = useRef<HTMLUListElement>(null);
  const root = useRef<HTMLDivElement>(null);
  const listId = useId();

  useEffect(() => {
    if (!open) return;
    setActive(index);
    list.current?.focus();
    const away = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", away);
    return () => document.removeEventListener("mousedown", away);
  }, [open, index]);

  const close = () => {
    setOpen(false);
    button.current?.focus();
  };
  const choose = (v: T) => {
    close();
    if (v !== value) onChange(v);
  };

  const onListKey = (e: React.KeyboardEvent) => {
    e.stopPropagation(); // keep the app's single-key shortcuts out of the menu
    const n = options.length;
    switch (e.key) {
      case "ArrowDown":
        setActive((i) => (i + 1) % n);
        break;
      case "ArrowUp":
        setActive((i) => (i - 1 + n) % n);
        break;
      case "Home":
        setActive(0);
        break;
      case "End":
        setActive(n - 1);
        break;
      case "Enter":
      case " ":
        choose(options[active].value);
        break;
      case "Escape":
      case "Tab":
        close();
        break;
      default: {
        const i = options.findIndex((o) => o.label.toLowerCase().startsWith(e.key.toLowerCase()));
        if (i < 0) return;
        setActive(i);
      }
    }
    e.preventDefault();
  };

  return (
    <div className="status-picker" ref={root}>
      <button
        ref={button}
        id={id}
        type="button"
        className={`status-button ${current.className}`}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        aria-label={`${name}: ${current.label}`}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown" || e.key === "ArrowUp") {
            e.preventDefault();
            e.stopPropagation();
            setOpen(true);
          }
        }}
      >
        <span className="status-icon" aria-hidden="true">
          {current.icon}
        </span>
        {current.label}
        <svg className="chevron" width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
          <path d="M2.5 4 5 6.5 7.5 4" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      {open && (
        <ul
          ref={list}
          id={listId}
          className="status-menu"
          role="listbox"
          aria-label={name}
          aria-activedescendant={`${listId}-${active}`}
          tabIndex={-1}
          onKeyDown={onListKey}
        >
          {options.map((o, i) => (
            <li
              key={o.label}
              id={`${listId}-${i}`}
              role="option"
              aria-selected={o.value === value}
              className={(i === active ? "active " : "") + o.className}
              onMouseEnter={() => setActive(i)}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => choose(o.value)}
            >
              <span className="status-icon" aria-hidden="true">
                {o.icon}
              </span>
              <span className="status-label">{o.label}</span>
              {o.value === value && (
                <svg className="check" width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">
                  <path d="M2.5 6.5 5 9l4.5-6" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
