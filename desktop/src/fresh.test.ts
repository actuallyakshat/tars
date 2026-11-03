import type { Capture } from "./api";
import { SEEN_FOR_MS, bellLit, isNew, markOpened, markSeen, prune, seed, type Fresh } from "./fresh";

const cap = (id: string, at = "2026-11-03T10:00:00Z"): Capture => ({ session_id: id, state: "unfiled", edits: 1, commits: 0, started_at: at });
const empty: Fresh = { marks: {}, bellAt: 0 };
const T = Date.parse("2026-11-03T12:00:00Z");

describe("new captured sessions", () => {
  it("are new until seen for 3 hours", () => {
    const c = cap("a");
    expect(isNew(c, empty, T, 1)).toBe(true);
    const f = markSeen(empty, ["a"], T, 1);
    expect(isNew(c, f, T + SEEN_FOR_MS - 1, 1)).toBe(true);
    expect(isNew(c, f, T + SEEN_FOR_MS, 1)).toBe(false);
  });

  it("stop being new on the next launch after being seen, but not before being seen", () => {
    const c = cap("a");
    expect(isNew(c, empty, T, 2)).toBe(true);
    expect(isNew(c, markSeen(empty, ["a"], T, 1), T, 2)).toBe(false);
  });

  it("stop being new once opened, and seeing again keeps the first time", () => {
    const c = cap("a");
    expect(isNew(c, markOpened(empty, ["a"]), T, 1)).toBe(false);
    const f = markSeen(empty, ["a"], T, 1);
    expect(markSeen(f, ["a"], T + 1000, 1)).toBe(f);
    expect(markSeen(markOpened(empty, ["a"]), ["a"], T, 1).marks.a).toEqual({ cleared: true });
  });

  it("start the first run with nothing new and the bell dark", () => {
    const list = [cap("a"), cap("b")];
    const f = seed(list, T);
    expect(list.some((c) => isNew(c, f, T, 1))).toBe(false);
    expect(bellLit(list, f)).toBe(false);
    expect(bellLit([...list, cap("c", "2026-11-03T12:05:00Z")], f)).toBe(true);
  });

  it("forget sessions no longer captured", () => {
    const f = markOpened(empty, ["a", "b"]);
    expect(Object.keys(prune(f, [cap("b")]).marks)).toEqual(["b"]);
    const same = markOpened(empty, ["b"]);
    expect(prune(same, [cap("b")])).toBe(same);
  });
});
