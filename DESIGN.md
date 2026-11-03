# Design philosophy

Rules for how the desktop app (`desktop/`) looks and is laid out. Follow them when adding or changing UI.

## Layout

### Pages use full width and are left-aligned

A page's content starts at the left edge of the main pane and uses all of its width. Don't center content in a narrow column.

- In code, pass `full` to `<Page>` (`desktop/src/components/Pages.tsx`). It removes the 640px max-width and auto margins of `.page-body` and keeps 40px side padding.
- This applies to every list, dashboard, and settings-style page: Unfiled, Todo lists, Settings, Legend, Repos, Usage.
- The one exception is the new/edit task form, a single focused input that stays a centered column.

**Why:** a centered column wastes most of a wide desktop window and moves the content's left edge whenever the window resizes. With a fixed left edge, pages line up with the sidebar and with each other, and the eye always knows where to start reading.

**Long text inside a full-width page** keeps its own readable measure (`max-width` in `ch`, e.g. `.page-lede` at 62ch, `.unfiled-prompt` at 72ch). That limits line length without centering anything: the page is full width and paragraphs aren't.
