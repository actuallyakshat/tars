// Which captured sessions still count as new. A session is new until it's
// opened, until the app is launched again after it was seen, or until 3 hours
// after it was first seen in the main pane, whichever comes first.
import type { Capture } from "./api";

export const FRESH_KEY = "tars.fresh";
const LAUNCH_KEY = "tars.launch";
export const SEEN_FOR_MS = 3 * 60 * 60 * 1000;

export interface Mark {
  /** When it was first seen in the main pane, and in which launch. */
  seenAt?: number;
  seenLaunch?: number;
  /** Opened, so no longer new. */
  cleared?: boolean;
}

export interface Fresh {
  marks: Record<string, Mark>;
  /** When the bell was last opened; captures since then light its dot. */
  bellAt: number;
}

/** When a session last did something: it ended, or it started. */
export const capturedAt = (c: Capture) => Date.parse(c.ended_at || c.started_at) || 0;

export function isNew(c: Capture, f: Fresh, now: number, launch: number) {
  const m = f.marks[c.session_id];
  if (!m) return true;
  if (m.cleared) return false;
  if (m.seenAt === undefined) return true;
  return m.seenLaunch === launch && now - m.seenAt < SEEN_FOR_MS;
}

/** Marks first sightings; one seen before keeps its first time. */
export function markSeen(f: Fresh, ids: string[], now: number, launch: number): Fresh {
  const add = ids.filter((id) => f.marks[id]?.seenAt === undefined && !f.marks[id]?.cleared);
  if (add.length === 0) return f;
  const marks = { ...f.marks };
  for (const id of add) marks[id] = { ...marks[id], seenAt: now, seenLaunch: launch };
  return { ...f, marks };
}

export function markOpened(f: Fresh, ids: string[]): Fresh {
  const add = ids.filter((id) => !f.marks[id]?.cleared);
  if (add.length === 0) return f;
  const marks = { ...f.marks };
  for (const id of add) marks[id] = { cleared: true };
  return { ...f, marks };
}

/** Forgets sessions no longer captured, so storage doesn't grow forever. */
export function prune(f: Fresh, captured: Capture[]): Fresh {
  const keep = new Set(captured.map((c) => c.session_id));
  const ids = Object.keys(f.marks);
  if (ids.every((id) => keep.has(id))) return f;
  return { ...f, marks: Object.fromEntries(ids.filter((id) => keep.has(id)).map((id) => [id, f.marks[id]])) };
}

export const bellLit = (captured: Capture[], f: Fresh) => captured.some((c) => capturedAt(c) > f.bellAt);

/**
 * The stored state, or null on the first run with this feature. The caller
 * then treats everything already captured as old news.
 */
export function loadFresh(): Fresh | null {
  try {
    const v = JSON.parse(localStorage.getItem(FRESH_KEY) ?? "null");
    if (v && typeof v === "object" && v.marks && typeof v.marks === "object") return { marks: v.marks, bellAt: Number(v.bellAt) || 0 };
  } catch {
    // Unreadable: start over.
  }
  return null;
}

export function saveFresh(f: Fresh) {
  localStorage.setItem(FRESH_KEY, JSON.stringify(f));
}

/** On the first run, nothing already captured is new and the bell is dark. */
export function seed(captured: Capture[], now: number): Fresh {
  return { marks: Object.fromEntries(captured.map((c) => [c.session_id, { cleared: true }])), bellAt: now };
}

// Counted once per process: the module loads once per app launch.
function nextLaunch() {
  const n = (Number(localStorage.getItem(LAUNCH_KEY)) || 0) + 1;
  localStorage.setItem(LAUNCH_KEY, String(n));
  return n;
}
export const LAUNCH = nextLaunch();
