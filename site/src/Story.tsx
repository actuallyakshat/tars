import { Reveal } from "./Reveal";

// The sections that explain what tars is for. Copy is written for an engineer
// who already runs Claude Code and wonders why this isn't just Jira.

export function Reports() {
  return (
    <section id="reports" className="section reports" aria-labelledby="reports-title">
      <Reveal>
        <h2 id="reports-title">Read what your agents did, not everything they said</h2>
        <p className="section-lede">
          Run five agents at once and you get five long terminal threads to scroll through. With tars, each agent
          finishes by writing a short report on its task: what changed, how it was checked, and what it needs from
          you. The full conversation is one command away if you want it.
        </p>
      </Reveal>
      <div className="compare">
        <Reveal className="compare-col">
          <p className="compare-label">In the terminal</p>
          <div className="transcript" aria-label="A long agent transcript, faded out">
            <p>I'll start by exploring the webhook sender to understand the current retry behaviour. Let me look at the delivery module first.</p>
            <p>I can see that deliveries are attempted once and any non-2xx response is treated as a permanent failure. Before changing this, let me check how the tests are structured and whether there's an existing backoff helper elsewhere in the codebase that I can reuse.</p>
            <p>I found a helper in pkg/retry, but it uses fixed intervals. I'll extend it to support exponential backoff with jitter, then wire it into the sender. Let me also make sure the alerting path is covered, since the ticket mentions alerting after the final failure.</p>
            <p>Now running the test suite to make sure nothing else depends on the old behaviour…</p>
            <p>All 212 tests pass, including 6 new ones. One thing I wasn't sure about: the ticket says to alert on-call, but there's also a #billing-alerts channel that the billing service already uses for…</p>
          </div>
        </Reveal>
        <Reveal className="compare-col" delay={0.12}>
          <p className="compare-label">In tars</p>
          <div className="report-card">
            <p className="report-head">
              <span className="report-id">#15</span> Add retry with backoff to webhook sender
            </p>
            <dl>
              <dt>Done</dt>
              <dd>Exponential backoff, 5 tries. Committed a41c9e2 on tars/15-webhook-retry.</dd>
              <dt>Verified</dt>
              <dd>make test: 212 passed (6 new)</dd>
              <dt>Needs you</dt>
              <dd className="ask">Alert #oncall or #billing-alerts after the last retry?</dd>
              <dt>Session</dt>
              <dd><code>claude --resume 8f3c21aa…</code></dd>
            </dl>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

export function NeedsYou() {
  return (
    <section className="section split" aria-labelledby="needs-title">
      <Reveal className="split-copy">
        <h2 id="needs-title">Every question waits in one place</h2>
        <p>
          When an agent can't decide something on its own, it doesn't guess and it doesn't stall a whole run. It
          leaves one clear question on the task and moves on.
        </p>
        <p>
          Open tars and the <strong>Needs you</strong> view lists every open question across all your agents.
          Answer in a few words, and the next agent that picks up the task reads your answer. When you want the
          full story, copy the <code>claude --resume</code> command and you're back in that exact session.
        </p>
      </Reveal>
      <Reveal className="split-visual" delay={0.1}>
        <ol className="qa">
          <li>
            <span className="qa-id">#15</span>
            <span className="qa-q">Alert #oncall or #billing-alerts after the last retry?</span>
            <span className="qa-a">billing-alerts</span>
          </li>
          <li>
            <span className="qa-id">#12</span>
            <span className="qa-q">Show export progress in the UI, or only in logs?</span>
            <span className="qa-a">UI, a simple progress bar</span>
          </li>
          <li>
            <span className="qa-id">#21</span>
            <span className="qa-q">Which repo should the event bus ADR live in?</span>
            <span className="qa-a pending">waiting for you</span>
          </li>
        </ol>
      </Reveal>
    </section>
  );
}

export function SteerLess() {
  return (
    <section className="section split" aria-labelledby="steer-title">
      <Reveal className="split-copy">
        <h2 id="steer-title">Steer less</h2>
        <p>
          CLAUDE.md is the same for every task. Real work isn't: "this ticket can't touch the public API" or "keep
          the old CSV format for this customer" only applies to one task.
        </p>
        <p>
          tars layers your preferences and hands the agent the right mix for the task in front of it. The most
          specific one wins, except your team's CLAUDE.md, which always comes first. Repo and personal preferences
          work today; per-ticket decisions, with clashes caught before the work starts, arrive with grilling.
        </p>
      </Reveal>
      <Reveal className="split-visual" delay={0.1}>
        <ol className="layers" aria-label="What the agent reads, highest priority first">
          <li>
            <span className="layer-name">CLAUDE.md</span>
            <span className="layer-note">Your team's hard rules. "Schema changes only through migrations."</span>
          </li>
          <li className="layer-ticket">
            <span className="layer-name">This ticket</span>
            <span className="layer-note">"Stream the CSV as one file. A 500-line PR is fine here."</span>
          </li>
          <li>
            <span className="layer-name">This repo</span>
            <span className="layer-note">"Exports paginate with cursors. Use testcontainers, not mocks."</span>
          </li>
          <li>
            <span className="layer-name">You</span>
            <span className="layer-note">"Small PRs. Plain commit messages."</span>
          </li>
        </ol>
      </Reveal>
    </section>
  );
}

const jiraRows: [string, string, string][] = [
  ["Who it's for", "Your team and your manager", "You and your agents"],
  ["What it keeps", "Status, assignee, comments", "Decisions, answered questions, agent reports"],
  ["Small personal tasks", "Rarely worth a ticket", "\"Ping Rahul\" is a task too"],
  ["Agent sessions", "Not tracked", "Linked to the task, resumable"],
  ["Questions from agents", "Lost in a terminal", "Waiting in Needs you"],
];

export function NotJira() {
  return (
    <section className="section" aria-labelledby="jira-title">
      <Reveal>
        <h2 id="jira-title">Why not just use Jira?</h2>
        <p className="section-lede">
          Keep using it. Jira tracks the work for your team. tars sits between the ticket and your agent: it holds
          the context the ticket never had, and the results your team doesn't need to read. Mention a ticket key and
          the agent pulls its details in; nothing has to move out of Jira.
        </p>
      </Reveal>
      <Reveal delay={0.1}>
        <table className="versus">
          <thead>
            <tr>
              <th scope="col"><span className="visually-hidden">Aspect</span></th>
              <th scope="col">Jira</th>
              <th scope="col">tars</th>
            </tr>
          </thead>
          <tbody>
            {jiraRows.map(([aspect, jira, tars]) => (
              <tr key={aspect}>
                <th scope="row">{aspect}</th>
                <td>{jira}</td>
                <td>{tars}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Reveal>
    </section>
  );
}

const habits: [string, string][] = [
  ["Dump everything as it comes", "Half-sentences are fine. Splitting, dates, Jira keys and repos are the agent's job."],
  ["Settle questions before work starts", "Ten minutes answering questions about a task saves a wasted night of guessing."],
  ["Keep tasks small", "One task, one reviewable branch. Smaller tasks mean shorter reports and easier reviews."],
  ["Answer from Needs you, not the terminal", "Your answer is stored on the task, so whichever agent picks it up next knows."],
  ["Start the day with the brief", "Read the reports, answer what's waiting, push only what you'd sign off on."],
];

export function Habits() {
  return (
    <section className="section" aria-labelledby="habits-title">
      <Reveal>
        <h2 id="habits-title">Getting the most out of it</h2>
        <p className="section-lede">Five habits that make the difference between a to-do list and a team.</p>
      </Reveal>
      <ol className="habits">
        {habits.map(([title, text], i) => (
          <Reveal as="li" key={title} delay={i * 0.05}>
            <h3>{title}</h3>
            <p>{text}</p>
          </Reveal>
        ))}
      </ol>
    </section>
  );
}

export function Status() {
  return (
    <section className="section" aria-labelledby="status-title">
      <Reveal>
        <h2 id="status-title">Where tars is today</h2>
        <p className="section-lede">tars is young and moving fast. Here's what works now and what's coming next.</p>
      </Reveal>
      <div className="status-grid">
        <Reveal className="status-col">
          <h3>Available now</h3>
          <ul>
            <li>Capture tasks in plain words from Claude Code or any terminal</li>
            <li>Short agent reports, with a Needs you view for open questions</li>
            <li>Every Claude session linked to its task, one command to resume</li>
            <li>Repo and personal profiles the agent reads before working</li>
            <li>Overnight sweeps into separate, tested branches; a morning brief</li>
            <li>A fast keyboard-driven terminal app</li>
          </ul>
        </Reveal>
        <Reveal className="status-col status-next" delay={0.1}>
          <h3>Coming next</h3>
          <ul>
            <li>
              <strong>Grilling:</strong> a relentless interview that settles every open question before a task is
              marked ready, built on{" "}
              <a href="https://github.com/mattpocock/skills">Matt Pocock's grilling skill</a> and shipped with tars
              (MIT, credited).
            </li>
            <li>
              <strong>Learning:</strong> tars notices your corrections and turns them into repo preferences on its
              own, and always tells you when it does.
            </li>
            <li>
              <strong>A macOS app</strong> for reading reports, answering questions and opening sessions.
            </li>
          </ul>
        </Reveal>
      </div>
    </section>
  );
}
