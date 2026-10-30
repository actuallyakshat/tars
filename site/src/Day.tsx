import type { ReactNode } from "react";
import { motion } from "motion/react";
import { Reveal } from "./Reveal";

type Step = {
  time: string;
  title: string;
  text: string;
  artifact: ReactNode;
};

const steps: Step[] = [
  {
    time: "2:14 pm",
    title: "Say it the way it comes to you",
    text: "Between meetings, dump whatever is on your mind. The agent splits it into separate tasks, picks out Jira keys, due dates and repos, and keeps track of who you're waiting on.",
    artifact: (
      <Chat>
        <You>/tars:tars need to fix the csv export timeout in billing (BILL-881) by friday, add retries to the webhook sender too, and rahul still owes me staging keys</You>
        <Agent>
          <p>Added 3 tasks:</p>
          <table className="chat-table">
            <tbody>
              <tr><td>#12</td><td>Fix CSV export timeout</td><td className="jira">BILL-881</td><td>due Fri</td></tr>
              <tr><td>#15</td><td>Add retry to webhook sender</td><td /><td>billing-svc</td></tr>
              <tr><td>#18</td><td>Get staging keys from Rahul</td><td /><td>follow up Thu</td></tr>
            </tbody>
          </table>
        </Agent>
      </Chat>
    ),
  },
  {
    time: "6:40 pm",
    title: "Get tomorrow's work ready",
    text: "Triage turns rough notes into tasks an agent can actually finish. It reads the Jira ticket, drafts acceptance criteria and asks only what it can't work out. Nothing is marked ready until you confirm it.",
    artifact: (
      <Chat>
        <You>/tars:tars triage my inbox</You>
        <Agent>
          <p>
            <strong>#15 Add retry to webhook sender.</strong> I read BILL-902 and drafted these criteria:
          </p>
          <ul>
            <li>Retries 5 times with exponential backoff</li>
            <li>Alerts on-call after the final failure</li>
          </ul>
          <p className="chat-choices">
            <span className="chat-choice on">Use these</span>
            <span className="chat-choice">Edit</span>
          </p>
          <p>
            <strong>#21 Write the ADR for the event bus.</strong> Which repo does this belong to?
          </p>
        </Agent>
      </Chat>
    ),
  },
  {
    time: "11:30 pm",
    title: "It works while you sleep",
    text: "Start a sweep and close the laptop. For each ready task, the agent opens a fresh git worktree, writes tests for the acceptance criteria, implements the change, runs your test suite and commits on its own branch. It never pushes.",
    artifact: (
      <pre className="term term-dark" aria-label="Sweep output">
        <span className="t-cmd">/tars:sweep jira only --yes</span>
        {"\n\n"}
        <span className="t-id">#15</span> Add retry to webhook sender{"\n"}
        {"    "}<span className="t-dim">worktree ../billing-svc.tars-15</span>{"\n"}
        {"    "}wrote 6 tests, implemented, <span className="t-ok">make test: 212 passed</span>{"\n"}
        {"    "}committed a41c9e2 on tars/15-webhook-retry <span className="t-review">→ in review</span>{"\n\n"}
        <span className="t-id">#9</span>  Paginate the audit log API{"\n"}
        {"    "}<span className="t-dim">worktree ../web.tars-9</span>{"\n"}
        {"    "}wrote 14 tests, implemented, <span className="t-ok">npm test: 388 passed</span>{"\n"}
        {"    "}committed 7be03d1 on tars/9-audit-pagination <span className="t-review">→ in review</span>{"\n\n"}
        <span className="t-id">#12</span> Fix CSV export timeout{"\n"}
        {"    "}<span className="t-warn">needs you:</span> should progress show in the UI or only in logs?{"\n"}
        {"    "}<span className="t-dim">left one question in Needs you and moved on</span>
      </pre>
    ),
  },
  {
    time: "8:05 am",
    title: "Review, then ship",
    text: "Coffee, then the brief. You see what the sweep produced, the questions it left you, what's due and who needs a nudge. Push the branches you like; the rest wait for you.",
    artifact: (
      <pre className="term" aria-label="Morning brief output">
        <span className="t-cmd">/tars:brief</span>
        {"\n\n"}
        <strong>Waiting for your review</strong>{"\n"}
        {"  "}<span className="t-id">#15</span> webhook retry      <span className="t-dim">+184 −12</span>   <span className="t-ok">212 passed</span>{"\n"}
        {"  "}<span className="t-id">#9</span>  audit pagination   <span className="t-dim">+341 −40</span>   <span className="t-ok">388 passed</span>{"\n\n"}
        <strong>Needs your answer</strong>{"\n"}
        {"  "}<span className="t-id">#12</span> Progress in the UI, or only in logs?{"\n\n"}
        <strong>Follow-ups due</strong>{"\n"}
        {"  "}<span className="t-id">#18</span> Rahul, staging keys. Draft: "Any update on the keys?"{"\n\n"}
        <strong>Suggested focus</strong>{"\n"}
        {"  "}Review #15 and #9, then answer #12.
      </pre>
    ),
  },
];

export function Day() {
  return (
    <section id="day" className="section day" aria-labelledby="day-title">
      <Reveal>
        <h2 id="day-title">A day with Tars</h2>
        <p className="section-lede">
          The list is only half of it. Here is how a day goes when the agent keeps it for you.
        </p>
      </Reveal>
      <ol className="steps">
        {steps.map((s) => (
          <Reveal as="li" key={s.time} className="step">
            <div className="step-copy">
              <time className="step-time">{s.time}</time>
              <h3>{s.title}</h3>
              <p>{s.text}</p>
            </div>
            <div className="step-artifact">{s.artifact}</div>
          </Reveal>
        ))}
      </ol>
    </section>
  );
}

function Chat({ children }: { children: ReactNode }) {
  return <div className="chat">{children}</div>;
}

// In view, your message lands first and the agent's reply follows a beat later,
// like a real exchange.
const inView = { once: true, margin: "0px 0px -15% 0px" } as const;

function You({ children }: { children: ReactNode }) {
  return (
    <motion.div
      className="chat-you"
      initial={{ opacity: 0, y: 6 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={inView}
      transition={{ duration: 0.4, delay: 0.15 }}
    >
      <span className="chat-prompt" aria-hidden="true">›</span>
      <span>{children}</span>
    </motion.div>
  );
}

function Agent({ children }: { children: ReactNode }) {
  return (
    <motion.div
      className="chat-agent"
      initial={{ opacity: 0, y: 6 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={inView}
      transition={{ duration: 0.45, delay: 0.75 }}
    >
      <motion.span
        className="chat-dot"
        aria-hidden="true"
        initial={{ scale: 0 }}
        whileInView={{ scale: 1 }}
        viewport={inView}
        transition={{ type: "spring", stiffness: 500, damping: 18, delay: 0.75 }}
      >
        ●
      </motion.span>
      <div>{children}</div>
    </motion.div>
  );
}
