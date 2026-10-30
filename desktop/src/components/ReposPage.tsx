import { useEffect, useRef, useState, type ReactNode } from "react";
import type { Repo, RepoDetail, Section } from "../api";
import { Page } from "./Pages";

// What agents know about each repo, and about you everywhere. Agents read the
// repo's own CLAUDE.md first; these fill in what it doesn't say.

export interface RepoActions {
  repos: Repo[];
  openTasks: Record<string, number>;
  onLoadRepo: (name: string) => Promise<RepoDetail | null>;
  onLoadProfile: () => Promise<Section[] | null>;
  onSetRepo: (name: string, path: string) => Promise<boolean>;
  onRemoveRepo: (name: string) => Promise<boolean>;
  onRenameRepo: (from: string, to: string) => Promise<boolean>;
  /** repo null means the personal profile. Empty text deletes the section. */
  onSetSection: (repo: string | null, section: string, text: string) => Promise<boolean>;
  onChooseFolder: () => Promise<string | null>;
  onReveal: (path: string) => void;
}

// Friendly names for the sections the CLI suggests. Anything else shows as written.
const SECTION_NAMES: Record<string, string> = {
  dod: "Definition of done",
  tests: "Tests",
  setup: "Setup",
  db: "Database",
  conventions: "Conventions",
  gotchas: "Gotchas",
  pr: "Pull requests",
  style: "Code style",
  review: "Review",
  commits: "Commits",
  avoid: "Avoid",
};

// What to suggest when adding a preference, in the order the CLI lists them.
const REPO_SUGGESTIONS = ["dod", "tests", "setup", "db", "conventions", "gotchas", "pr"];
const PROFILE_SUGGESTIONS = ["style", "review", "commits", "avoid"];

const sectionTitle = (name: string) => SECTION_NAMES[name] ?? name.charAt(0).toUpperCase() + name.slice(1).replace(/[-_]/g, " ");
const basename = (p: string) => p.replace(/\/+$/, "").split("/").pop() ?? "";
const shortPath = (p: string) => p.replace(/^\/Users\/[^/]+(?=\/|$)/, "~");

const EVERYWHERE = "\u0000everywhere";

export function ReposPage({ onBack, ...a }: RepoActions & { onBack: () => void }) {
  const [current, setCurrent] = useState<string>(() => a.repos[0]?.name ?? EVERYWHERE);
  const [adding, setAdding] = useState(false);

  // If the shown repo is removed, fall back to the first one.
  useEffect(() => {
    if (current !== EVERYWHERE && !a.repos.some((r) => r.name === current)) setCurrent(a.repos[0]?.name ?? EVERYWHERE);
  }, [a.repos, current]);

  return (
    <Page title="Repos" onBack={onBack} full>
      <p className="lede">What your agents know about each repo, on top of its own CLAUDE.md.</p>
      <div className="repos">
        <nav className="repos-nav" aria-label="Repos">
          <button
            className={"repos-nav-item" + (current === EVERYWHERE ? " on" : "")}
            aria-current={current === EVERYWHERE ? "page" : undefined}
            onClick={() => {
              setCurrent(EVERYWHERE);
              setAdding(false);
            }}
          >
            <span className="repos-nav-name">Everywhere</span>
            <span className="repos-nav-sub">Your personal profile</span>
          </button>
          <div className="repos-nav-rule" />
          {a.repos.map((r) => (
            <button
              key={r.name}
              className={"repos-nav-item" + (current === r.name && !adding ? " on" : "")}
              aria-current={current === r.name && !adding ? "page" : undefined}
              onClick={() => {
                setCurrent(r.name);
                setAdding(false);
              }}
            >
              <span className="repos-nav-name">{r.name}</span>
              <span className="repos-nav-sub mono">{r.path ? shortPath(r.path) : "No folder"}</span>
              {!!a.openTasks[r.name] && (
                <span className="repos-nav-count" aria-label={`${a.openTasks[r.name]} open tasks`}>
                  {a.openTasks[r.name]}
                </span>
              )}
            </button>
          ))}
          <button className={"repos-add-button" + (adding ? " on" : "")} onClick={() => setAdding(true)}>
            <span aria-hidden="true">+</span> Add a repo
          </button>
        </nav>

        <div className="repos-main">
          {adding ? (
            <AddRepo {...a} onAdded={(name) => {
              setAdding(false);
              setCurrent(name);
            }} />
          ) : current === EVERYWHERE ? (
            <Everywhere onLoad={a.onLoadProfile} onSetSection={a.onSetSection} />
          ) : (
            <RepoView key={current} name={current} {...a} onRenamed={setCurrent} />
          )}
        </div>
      </div>
    </Page>
  );
}

function useLoad<T>(load: () => Promise<T | null>, deps: unknown[]) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let live = true;
    setLoading(true);
    load().then((d) => {
      if (!live) return;
      setData(d);
      setLoading(false);
    });
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
  return { data, loading };
}

function Everywhere({ onLoad, onSetSection }: Pick<RepoActions, "onSetSection"> & { onLoad: () => Promise<Section[] | null> }) {
  const [version, setVersion] = useState(0);
  const { data, loading } = useLoad(onLoad, [version]);
  return (
    <section aria-label="Everywhere">
      <header className="repo-head">
        <h2>Everywhere</h2>
        <p className="muted">How you like to work, in every repo. A repo's CLAUDE.md and its profile win where they disagree.</p>
      </header>
      <Sections
        loading={loading}
        sections={data ?? []}
        suggestions={PROFILE_SUGGESTIONS}
        onSave={async (section, text) => {
          const ok = await onSetSection(null, section, text);
          if (ok) setVersion((v) => v + 1);
          return ok;
        }}
        empty={
          <>
            Nothing yet. Agents add to it when you correct them, or set a section yourself:{" "}
            <code>tars profile set style -x "small, focused diffs"</code>
          </>
        }
      />
    </section>
  );
}

function RepoView({
  name,
  onLoadRepo,
  onSetRepo,
  onRemoveRepo,
  onRenameRepo,
  onSetSection,
  onChooseFolder,
  onReveal,
  openTasks,
  onRenamed,
}: RepoActions & { name: string; onRenamed: (name: string) => void }) {
  const [version, setVersion] = useState(0);
  const { data, loading } = useLoad(() => onLoadRepo(name), [name, version]);
  const [confirming, setConfirming] = useState(false);
  const [renaming, setRenaming] = useState(false);

  const repoint = async () => {
    const dir = await onChooseFolder();
    if (dir && (await onSetRepo(name, dir))) setVersion((v) => v + 1);
  };

  return (
    <section aria-label={name}>
      <header className="repo-head">
        <div className="repo-head-row">
          {renaming ? (
            <RenameField
              name={name}
              onCancel={() => setRenaming(false)}
              onSave={async (to) => {
                const ok = await onRenameRepo(name, to);
                if (ok) {
                  setRenaming(false);
                  onRenamed(to);
                }
              }}
            />
          ) : (
            <>
              <h2>{name}</h2>
              {!!openTasks[name] && <span className="repo-badge">{openTasks[name]} open</span>}
              <button className="link rename-link" onClick={() => setRenaming(true)} aria-label={`Rename ${name}`}>
                Rename
              </button>
            </>
          )}
        </div>
        <div className="repo-folder">
          {data?.path ? (
            <button className="link mono" title={`Show ${data.path} in Finder`} onClick={() => onReveal(data.path!)}>
              {shortPath(data.path)}
            </button>
          ) : (
            <span className="muted">No folder set, so agents can't tell when they're in this repo.</span>
          )}
          {confirming ? (
            <span className="repo-actions">
              <span className="muted">Delete this profile and its preferences?</span>
              <button className="ghost" onClick={() => setConfirming(false)}>
                Keep
              </button>
              <button className="danger" onClick={() => void onRemoveRepo(name)}>
                Delete {name}
              </button>
            </span>
          ) : (
            <span className="repo-actions">
              <button className="ghost" onClick={() => void repoint()}>
                {data?.path ? "Change folder…" : "Set folder…"}
              </button>
              <button className="ghost" onClick={() => setConfirming(true)} aria-label={`Remove ${name}`}>
                Remove
              </button>
            </span>
          )}
        </div>
      </header>
      <Sections
        loading={loading}
        sections={data?.sections ?? []}
        suggestions={REPO_SUGGESTIONS}
        onSave={async (section, text) => {
          const ok = await onSetSection(name, section, text);
          if (ok) setVersion((v) => v + 1);
          return ok;
        }}
        empty={
          <>
            No preferences for {name} yet. Agents add them as they learn the repo, or set one yourself:{" "}
            <code>tars repo set {name} -S tests -x "make test"</code>
          </>
        }
      />
    </section>
  );
}

function Sections({
  loading,
  sections,
  empty,
  suggestions,
  onSave,
}: {
  loading: boolean;
  sections: Section[];
  empty: ReactNode;
  suggestions: string[];
  onSave: (section: string, text: string) => Promise<boolean>;
}) {
  const [adding, setAdding] = useState(false);
  if (loading) return <p className="muted repo-loading">Loading…</p>;
  const unused = suggestions.filter((n) => !sections.some((s) => s.name === n));
  return (
    <>
      {sections.length === 0 && !adding && <p className="repo-empty">{empty}</p>}
      <div className="pref-grid">
        {sections.map((s) => (
          <PrefCard key={s.name} section={s} onSave={(text) => onSave(s.name, text)} />
        ))}
        {adding ? (
          <PrefEditor
            title="New preference"
            suggestions={unused}
            onCancel={() => setAdding(false)}
            onSave={async (name, text) => {
              const ok = await onSave(name, text);
              if (ok) setAdding(false);
              return ok;
            }}
          />
        ) : (
          <button className="pref-add" onClick={() => setAdding(true)}>
            <span aria-hidden="true">+</span> Add a preference
          </button>
        )}
      </div>
    </>
  );
}

function PrefCard({ section, onSave }: { section: Section; onSave: (text: string) => Promise<boolean> }) {
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const title = sectionTitle(section.name);
  if (editing) {
    return (
      <PrefEditor
        title={title}
        name={section.name}
        initial={section.content}
        onCancel={() => setEditing(false)}
        onSave={async (_, text) => {
          const ok = await onSave(text);
          if (ok) setEditing(false);
          return ok;
        }}
      />
    );
  }
  return (
    <article className="pref-card" aria-label={title}>
      <h3>
        <span>
          {title}
          {SECTION_NAMES[section.name] && <span className="pref-key mono">{section.name}</span>}
        </span>
        <span className="pref-actions">
          {confirming ? (
            <>
              <button className="link" onClick={() => setConfirming(false)}>
                Keep
              </button>
              <button className="link danger-link" onClick={() => void onSave("")} aria-label={`Delete ${title}`}>
                Delete
              </button>
            </>
          ) : (
            <>
              <button className="link" onClick={() => setEditing(true)} aria-label={`Edit ${title}`}>
                Edit
              </button>
              <button className="link" onClick={() => setConfirming(true)} aria-label={`Remove ${title}`}>
                Remove
              </button>
            </>
          )}
        </span>
      </h3>
      <Prose text={section.content} />
    </article>
  );
}

// Writes or rewrites one preference. ⌘↩ saves, Esc cancels.
function PrefEditor({
  title,
  name: fixedName,
  initial = "",
  suggestions = [],
  onSave,
  onCancel,
}: {
  title: string;
  name?: string;
  initial?: string;
  suggestions?: string[];
  onSave: (name: string, text: string) => Promise<boolean>;
  onCancel: () => void;
}) {
  const [name, setName] = useState(fixedName ?? "");
  const [text, setText] = useState(initial);
  const [busy, setBusy] = useState(false);
  const area = useRef<HTMLTextAreaElement>(null);
  const nameRef = useRef<HTMLInputElement>(null);
  useEffect(() => (fixedName ? area.current?.focus() : nameRef.current?.focus()), [fixedName]);
  const ready = name.trim() && text.trim() && !busy;

  const save = async () => {
    if (!ready) return;
    setBusy(true);
    await onSave(name.trim().toLowerCase(), text);
    setBusy(false);
  };

  return (
    <form
      className="pref-card editing"
      aria-label={`Editing ${title}`}
      onSubmit={(e) => {
        e.preventDefault();
        void save();
      }}
      onKeyDown={(e) => {
        e.stopPropagation(); // keep Esc here instead of leaving the page
        if (e.key === "Escape") onCancel();
        else if (e.key === "Enter" && e.metaKey) {
          e.preventDefault();
          void save();
        }
      }}
    >
      {fixedName ? (
        <h3>{title}</h3>
      ) : (
        <>
          <input
            ref={nameRef}
            className="pref-name-input"
            aria-label="Preference name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Name, e.g. tests"
            spellCheck={false}
          />
          {suggestions.length > 0 && (
            <span className="chips">
              {suggestions.map((n) => (
                <button type="button" key={n} className={"chip" + (name === n ? " on" : "")} onClick={() => setName(n)}>
                  {sectionTitle(n)}
                </button>
              ))}
            </span>
          )}
        </>
      )}
      <textarea
        ref={area}
        aria-label={`${title} text`}
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={Math.min(14, Math.max(4, text.split("\n").length + 1))}
        placeholder={"What agents should know. Lines starting with - become a list."}
      />
      <div className="pref-editor-actions">
        <button type="button" className="ghost" onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className="primary" disabled={!ready}>
          Save <kbd aria-hidden="true">⌘↩</kbd>
        </button>
      </div>
    </form>
  );
}

// Swaps the repo's heading for a text field. Enter saves, Esc cancels.
function RenameField({ name, onSave, onCancel }: { name: string; onSave: (to: string) => Promise<void>; onCancel: () => void }) {
  const [value, setValue] = useState(name);
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    ref.current?.focus();
    ref.current?.select();
  }, []);
  return (
    <form
      className="rename-form"
      onSubmit={(e) => {
        e.preventDefault();
        if (value.trim() && value.trim() !== name) void onSave(value.trim());
        else onCancel();
      }}
    >
      <input
        ref={ref}
        className="rename-input"
        aria-label="Repo name"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            e.stopPropagation();
            onCancel();
          }
        }}
        spellCheck={false}
      />
      <button type="submit" className="primary">
        Rename
      </button>
      <button type="button" className="ghost" onClick={onCancel}>
        Cancel
      </button>
    </form>
  );
}

// Just enough Markdown for preference notes: "- " bullets, paragraphs and
// `code`. Anything else shows as written.
function Prose({ text }: { text: string }) {
  const blocks: ReactNode[] = [];
  let bullets: string[] = [];
  let para: string[] = [];
  const flush = () => {
    if (bullets.length) {
      blocks.push(
        <ul key={blocks.length}>
          {bullets.map((b, i) => (
            <li key={i}>{inline(b)}</li>
          ))}
        </ul>,
      );
      bullets = [];
    }
    if (para.length) {
      blocks.push(<p key={blocks.length}>{inline(para.join(" "))}</p>);
      para = [];
    }
  };
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    const bullet = line.match(/^[-*•]\s+(.*)$/);
    if (!line) flush();
    else if (bullet) {
      if (para.length) flush();
      bullets.push(bullet[1]);
    } else {
      if (bullets.length) flush();
      para.push(line);
    }
  }
  flush();
  return <div className="prose">{blocks}</div>;
}

function inline(s: string) {
  return s.split(/(`[^`]+`)/).map((part, i) => (part.startsWith("`") && part.endsWith("`") && part.length > 2 ? <code key={i}>{part.slice(1, -1)}</code> : part));
}

function AddRepo({ onSetRepo, onChooseFolder, onAdded }: RepoActions & { onAdded: (name: string) => void }) {
  const [name, setName] = useState("");
  const [path, setPath] = useState("");
  const [busy, setBusy] = useState(false);

  const add = async () => {
    if (!name.trim() || !path.trim() || busy) return;
    setBusy(true);
    const ok = await onSetRepo(name.trim(), path.trim());
    setBusy(false);
    if (ok) onAdded(name.trim());
  };

  const browse = async () => {
    const dir = await onChooseFolder();
    if (!dir) return;
    setPath(dir);
    if (!name.trim()) setName(basename(dir));
  };

  return (
    <section aria-label="Add a repo">
      <header className="repo-head">
        <h2>Add a repo</h2>
        <p className="muted">Tell tars where the repo lives. Agents working inside that folder get its profile, and new tasks can pick it.</p>
      </header>
      <form
        className="repo-add"
        onSubmit={(e) => {
          e.preventDefault();
          void add();
        }}
      >
        <div className="repo-add-row">
          <label className="field">
            <span className="field-label">Folder</span>
            <span className="path-field">
              <input className="mono" value={path} onChange={(e) => setPath(e.target.value)} placeholder="~/code/billing-svc" />
              <button type="button" className="ghost" onClick={() => void browse()}>
                Choose…
              </button>
            </span>
          </label>
          <label className="field">
            <span className="field-label">Name</span>
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="billing-svc" />
          </label>
        </div>
        <div className="settings-save">
          <button type="submit" className="primary" disabled={!name.trim() || !path.trim() || busy}>
            Add repo
          </button>
        </div>
      </form>
    </section>
  );
}
