import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

vi.mock("@tauri-apps/api/core", () => ({ invoke: vi.fn() }));

// jsdom has no layout, so scrolling is a no-op.
Element.prototype.scrollIntoView = function () {};

afterEach(() => {
  localStorage.clear();
});
