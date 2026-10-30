import { useEffect, useRef, useState } from "react";

interface Props {
  label: string;
  placeholder: string;
  submitLabel: string;
  onSubmit: (text: string) => Promise<boolean>;
  onCancel: () => void;
}

/** A small inline text box. ⌘↩ or ↩ submits, Esc cancels. */
export function Composer({ label, placeholder, submitLabel, onSubmit, onCancel }: Props) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const ref = useRef<HTMLTextAreaElement>(null);
  useEffect(() => ref.current?.focus(), []);

  const submit = async () => {
    if (!text.trim() || busy) return;
    setBusy(true);
    const ok = await onSubmit(text.trim());
    setBusy(false);
    if (ok) setText("");
  };

  return (
    <form
      className="composer"
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <textarea
        ref={ref}
        aria-label={label}
        placeholder={placeholder}
        value={text}
        rows={2}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            e.preventDefault();
            onCancel();
          } else if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            void submit();
          }
        }}
      />
      <div className="composer-actions">
        <span className="hint">↩ to send · ⇧↩ new line · esc to cancel</span>
        <button type="button" className="ghost" onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className="primary" disabled={!text.trim() || busy}>
          {submitLabel}
        </button>
      </div>
    </form>
  );
}
