import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "../App";
import { formatMoney, money } from "../currency";
import { fakeBackend, sampleUsage } from "../test/backend";

async function openUsage() {
  const user = userEvent.setup();
  render(<App />);
  const nav = await screen.findByRole("navigation", { name: "Views" });
  await user.click(within(nav).getByRole("button", { name: /Usage/ }));
  await screen.findByText("Total cost");
  return user;
}

const tile = (label: string) => screen.getByText(label).closest(".usage-tile") as HTMLElement;

beforeEach(() => localStorage.clear());

describe("currency helpers", () => {
  it("formats rupees in lakhs and keeps tiny amounts readable", () => {
    expect(formatMoney(123456.7, "INR")).toBe("₹1,23,457");
    expect(formatMoney(12.5, "USD")).toBe("$12.50");
    expect(formatMoney(0.004, "USD")).toBe("$0.004");
  });

  it("converts at the fetched rate and falls back to USD without one", () => {
    const fx = sampleUsage().fx;
    expect(money(fx, "INR").format(10)).toBe("₹949.10");
    const none = { ...fx, rates: {}, status: "unavailable" as const };
    expect(money(none, "INR")).toMatchObject({ currency: "USD" });
    expect(money(none, "INR").format(10)).toBe("$10.00");
  });
});

describe("Usage page", () => {
  it("shows the period's totals and breakdowns", async () => {
    const b = fakeBackend();
    await openUsage();
    // The open task's detail asks for its own cost too.
    expect(b.callsTo("usage_summary")).toContainEqual({ days: 30, task: null });
    const total = sampleUsage(30).total.cost_usd;
    expect(within(tile("Total cost")).getByText(formatMoney(total, "USD"))).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "By repo" })).toHaveTextContent("billing-svc");
    expect(screen.getByRole("region", { name: "By model" })).toHaveTextContent("claude-opus-5-5");
    expect(screen.getByText(/1 USD = ₹94.91/)).toBeInTheDocument();
    expect(screen.getByText(/No price yet for mystery-model/)).toBeInTheDocument();
  });

  it("switches to rupees at the live rate and remembers it", async () => {
    fakeBackend();
    const user = await openUsage();
    await user.click(screen.getByRole("radio", { name: "₹ INR" }));
    const total = sampleUsage(30).total.cost_usd;
    expect(within(tile("Total cost")).getByText(formatMoney(total * 94.91, "INR"))).toBeInTheDocument();
    expect(localStorage.getItem("tars.currency")).toBe("INR");
  });

  it("refetches for another period", async () => {
    const b = fakeBackend();
    const user = await openUsage();
    await user.click(screen.getByRole("radio", { name: "7 days" }));
    await waitFor(() => expect(b.callsTo("usage_summary").at(-1)).toEqual({ days: 7, task: null }));
  });

  it("won't offer rupees without an exchange rate", async () => {
    const b = fakeBackend();
    b.usage = (days) => ({ ...sampleUsage(days), fx: { base: "USD", rates: {}, status: "unavailable" } });
    await openUsage();
    expect(screen.getByRole("radio", { name: "₹ INR" })).toBeDisabled();
    expect(screen.getByText(/Exchange rates couldn't be fetched/)).toBeInTheDocument();
  });

  it("opens a task from its row", async () => {
    fakeBackend();
    const user = await openUsage();
    await user.click(within(screen.getByRole("region", { name: "By task" })).getByRole("button", { name: /Add retry to webhook sender/ }));
    expect(await screen.findByRole("heading", { name: "Add retry to webhook sender" })).toBeInTheDocument();
  });

  it("explains a failure and can retry", async () => {
    const b = fakeBackend();
    b.fail.usage_summary = { kind: "failed", message: "tars predates the Usage page" };
    const user = userEvent.setup();
    render(<App />);
    const nav = await screen.findByRole("navigation", { name: "Views" });
    await user.click(within(nav).getByRole("button", { name: /Usage/ }));
    expect(await screen.findByRole("alert")).toHaveTextContent("predates the Usage page");
    delete b.fail.usage_summary;
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("Total cost")).toBeInTheDocument();
  });
});
