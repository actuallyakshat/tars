import { useEffect, useRef, useState } from "react";
import type { CliError } from "../api";
import { Mark } from "../Mark";

// Asks before something that can't be undone. Focus starts on Cancel, so a
// stray Enter keeps the task; Esc or a click outside also cancels.
export function ConfirmDialog({
  title,
  body,
  confirmLabel,
  onConfirm,
  onCancel,
}: {
  title: string;
  body: string;
  confirmLabel: string;
  onConfirm: () => Promise<unknown>;
  onCancel: () => void;
}) {
  const cancel = useRef<HTMLButtonElement>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => cancel.current?.focus(), []);
  return (
    <div className="confirm-backdrop" onMouseDown={() => !busy && onCancel()}>
      <div
        className="confirm"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
        aria-describedby="confirm-body"
        onMouseDown={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          e.stopPropagation();
          if (e.key === "Escape" && !busy) onCancel();
          // Keep Tab inside the dialog.
          if (e.key === "Tab") {
            const buttons = [...e.currentTarget.querySelectorAll("button")];
            const i = buttons.indexOf(document.activeElement as HTMLButtonElement);
            e.preventDefault();
            buttons[(i + (e.shiftKey ? buttons.length - 1 : 1)) % buttons.length]?.focus();
          }
        }}
      >
        <h2 id="confirm-title">{title}</h2>
        <p id="confirm-body">{body}</p>
        <div className="confirm-actions">
          <button ref={cancel} className="ghost" onClick={onCancel} disabled={busy}>
            Cancel
          </button>
          <button
            className="danger"
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              await onConfirm();
            }}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}

export function CliProblem({ error, onRetry }: { error: CliError; onRetry: () => void }) {
  const title = error.kind === "too_old" ? "Your tars CLI needs an update" : "tars needs its command-line tool";
  return (
    <main className="problem" role="alert">
      <div className="drag problem-top" data-tauri-drag-region />
      <Mark size={40} />
      <h1>{title}</h1>
      <p>{error.message}</p>
      <p className="hint">
        The app reads and changes tasks only through <span className="mono">tars</span>, and needs a build that has{" "}
        <span className="mono">tars report</span> (newer than 0.1.3). Set <span className="mono">TARS_BIN</span> to use
        a specific binary.
      </p>
      <button className="primary" onClick={onRetry}>
        Try again
      </button>
    </main>
  );
}

export function OutsideApp() {
  return (
    <main className="problem" role="alert">
      <Mark size={40} />
      <h1>Open this in the Tars app</h1>
      <p>
        This page is the app's interface, and it only works inside the Tars window. A browser tab can't reach the{" "}
        <span className="mono">tars</span> command-line tool.
      </p>
      <p className="hint">
        Close this tab and switch to the Tars window. If it isn't open, run{" "}
        <span className="mono">npx tauri dev</span> in the <span className="mono">desktop</span> folder.
      </p>
    </main>
  );
}
