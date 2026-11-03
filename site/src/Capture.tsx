import { motion } from "motion/react";

// The hero diagram: sessions on the left, what Tars did with each on the right.
// It's an explanation, not a product screen, so it carries no window chrome.

type Outcome = "linked" | "asked" | "unfiled" | "dropped";

type Row = {
  repo: string;
  branch: string;
  prompt: string;
  outcome: Outcome;
  result: string;
  why: string;
};

const rows: Row[] = [
  {
    repo: "billing-svc", branch: "tars/15-webhook-retry", prompt: "alert after the last failed retry",
    outcome: "linked", result: "Linked to #15", why: "The branch is named for the task",
  },
  {
    repo: "web", branch: "feat/audit-pagination", prompt: "page size should default to 100",
    outcome: "linked", result: "Linked to #9", why: "This branch is already on #9",
  },
  {
    repo: "billing-svc", branch: "main", prompt: "BILL-881 export still times out for Acme",
    outcome: "linked", result: "Linked to #12", why: "Jira key BILL-881 in the prompt",
  },
  {
    repo: "platform", branch: "flags-cleanup", prompt: "remove the old checkout feature flags",
    outcome: "asked", result: "Claude asked you once", why: "Is this #23 Clean up stale feature flags, or none?",
  },
  {
    repo: "web", branch: "fix-footer", prompt: "fix the typo in the footer",
    outcome: "unfiled", result: "Held as Unfiled", why: "Changed 2 files, matched no task",
  },
  {
    repo: "api", branch: "main", prompt: "why does the rate limiter use redis?",
    outcome: "dropped", result: "Nothing kept", why: "Only read code and answered a question",
  },
];

const ease = [0.2, 0.7, 0.2, 1] as const;

const row = {
  hidden: { opacity: 0 },
  shown: { opacity: 1, transition: { duration: 0.3, when: "beforeChildren", staggerChildren: 0.22 } },
} as const;

const part = {
  hidden: { opacity: 0, x: -6 },
  shown: { opacity: 1, x: 0, transition: { duration: 0.35, ease } },
} as const;

const wire = {
  hidden: { scaleX: 0 },
  shown: { scaleX: 1, transition: { duration: 0.35, ease } },
} as const;

export function Capture() {
  return (
    <figure className="capture">
      <div className="capture-head" aria-hidden="true">
        <span>Session and its first prompt</span>
        <span>What Tars did</span>
      </div>
      <motion.ol
        className="capture-rows"
        initial="hidden"
        animate="shown"
        transition={{ staggerChildren: 0.42, delayChildren: 0.5 }}
      >
        {rows.map((r) => (
          <motion.li key={r.prompt} className={`capture-row is-${r.outcome}`} variants={row}>
            <motion.div className="capture-session" variants={part}>
              <span className="capture-where">
                <span>{r.repo}</span> <span className="capture-branch">{r.branch}</span>
              </span>
              <span className="capture-prompt">{r.prompt}</span>
            </motion.div>
            <motion.span className="capture-wire" variants={wire} aria-hidden="true" />
            <motion.div className="capture-outcome" variants={part}>
              <span className="capture-result">
                <span className="capture-dot" aria-hidden="true" />
                {r.result}
              </span>
              <span className="capture-why">{r.why}</span>
            </motion.div>
          </motion.li>
        ))}
      </motion.ol>
      <figcaption className="capture-caption">
        Six sessions from one day. Three linked themselves, one took a single question, one waits for you to sort,
        and one left nothing behind.
      </figcaption>
    </figure>
  );
}
