import type { CliError } from "../api";
import { Mark } from "../Mark";

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
