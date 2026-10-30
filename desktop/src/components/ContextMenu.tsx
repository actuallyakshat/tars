import { useEffect, useLayoutEffect, useRef, useState } from "react";

export interface MenuItem {
  label: string;
  keys?: string;
  danger?: boolean;
  run: () => void;
}

// A right-click menu at the pointer. ↑↓ move, Enter chooses, Esc or a click
// elsewhere closes it.
export function ContextMenu({ x, y, label, items, onClose }: { x: number; y: number; label: string; items: MenuItem[]; onClose: () => void }) {
  const ref = useRef<HTMLUListElement>(null);
  const [active, setActive] = useState(0);
  const [pos, setPos] = useState({ left: x, top: y });

  // Keep the menu inside the window.
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    setPos({
      left: Math.max(4, Math.min(x, window.innerWidth - r.width - 4)),
      top: Math.max(4, Math.min(y, window.innerHeight - r.height - 4)),
    });
  }, [x, y]);

  useEffect(() => {
    ref.current?.focus();
    const away = (e: Event) => {
      if (!ref.current?.contains(e.target as Node)) onClose();
    };
    document.addEventListener("mousedown", away);
    window.addEventListener("blur", onClose);
    window.addEventListener("resize", onClose);
    return () => {
      document.removeEventListener("mousedown", away);
      window.removeEventListener("blur", onClose);
      window.removeEventListener("resize", onClose);
    };
  }, [onClose]);

  const choose = (item: MenuItem) => {
    onClose();
    item.run();
  };

  return (
    <ul
      ref={ref}
      className="context-menu"
      role="menu"
      aria-label={label}
      tabIndex={-1}
      style={pos}
      onContextMenu={(e) => e.preventDefault()}
      onKeyDown={(e) => {
        e.stopPropagation();
        const n = items.length;
        if (e.key === "ArrowDown") setActive((i) => (i + 1) % n);
        else if (e.key === "ArrowUp") setActive((i) => (i - 1 + n) % n);
        else if (e.key === "Enter" || e.key === " ") choose(items[active]);
        else if (e.key === "Escape" || e.key === "Tab") onClose();
        else return;
        e.preventDefault();
      }}
    >
      {items.map((item, i) => (
        <li
          key={item.label}
          role="menuitem"
          className={"context-item" + (i === active ? " active" : "") + (item.danger ? " danger-item" : "")}
          onMouseEnter={() => setActive(i)}
          onClick={() => choose(item)}
        >
          <span>{item.label}</span>
          {item.keys && <kbd aria-hidden="true">{item.keys}</kbd>}
        </li>
      ))}
    </ul>
  );
}
