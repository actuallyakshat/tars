import { useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { TuiDemo } from "./TuiDemo";
import { Day } from "./Day";
import { Habits, NeedsYou, NotJira, Reports, Status, SteerLess } from "./Story";
import { Mark } from "./Mark";
import { Reveal, press } from "./Reveal";

// Update when the repo is published under its final GitHub owner.
const REPO = "actuallyakshat/tars";
const OWNER = REPO.split("/")[0];

// The hero's load-in: one short staggered sequence.
const heroItem = {
  hidden: { opacity: 0, y: 12, filter: "blur(4px)" },
  shown: { opacity: 1, y: 0, filter: "blur(0px)", transition: { duration: 0.7, ease: [0.2, 0.7, 0.2, 1] } },
} as const;

export function App() {
  return (
    <>
      <a className="skip" href="#main">Skip to content</a>
      <header className="nav">
        <a className="logo" href="#top" aria-label="Tars home">
          <Mark size={24} />
          <span>Tars</span>
        </a>
        <nav aria-label="Primary">
          <a href="#reports">How it works</a>
          <a href="#install">Install</a>
          <a href={`https://github.com/${REPO}`}>GitHub</a>
        </nav>
      </header>

      <main id="main">
        <motion.section
          id="top"
          className="hero"
          initial="hidden"
          animate="shown"
          transition={{ staggerChildren: 0.08 }}
        >
          <div className="hero-copy">
            <motion.div variants={heroItem}>
              <motion.div
                className="hero-mark"
                whileHover={{ rotate: -8, scale: 1.04 }}
                transition={{ type: "spring", stiffness: 300, damping: 14 }}
              >
                <Mark size={64} />
              </motion.div>
            </motion.div>
            <motion.h1 variants={heroItem}>Stay on top of every agent, without reading every transcript</motion.h1>
            <motion.p className="lede" variants={heroItem}>
              Tars is the task list you and Claude share. Agents report back in a few lines: what they did, how they
              checked it, what they need from you. Questions wait in one place, every session is a command away, and
              you steer less because Tars remembers how you want things done.
            </motion.p>
            <motion.div className="ctas" variants={heroItem}>
              <motion.a className="btn btn-primary" href="#install" {...press}>
                <PromptIcon />
                Install Tars
              </motion.a>
              <motion.a className="btn btn-quiet" href="#reports" {...press}>
                See how it works
                <ChevronDownIcon />
              </motion.a>
            </motion.div>
          </div>
          <motion.div className="hero-demo" variants={heroItem}>
            <TuiDemo />
          </motion.div>
        </motion.section>

        <Reports />
        <NeedsYou />
        <SteerLess />
        <NotJira />
        <Day />
        <Habits />
        <Status />

        <section className="section charge" aria-labelledby="charge-title">
          <Reveal>
            <h2 id="charge-title">You stay in charge</h2>
          </Reveal>
          <dl className="rules">
            <Reveal delay={0.00}>
              <dt>Runs only when you ask</dt>
              <dd>Every command is started by you. The agent never reads your list or picks up work on its own.</dd>
            </Reveal>
            <Reveal delay={0.05}>
              <dt>Never pushes</dt>
              <dd>Sweeps commit to local branches in separate worktrees. Your main checkout isn't touched.</dd>
            </Reveal>
            <Reveal delay={0.10}>
              <dt>Shows its plan first</dt>
              <dd>A sweep lists every task, branch and test command before it starts, unless you pass --yes.</dd>
            </Reveal>
            <Reveal delay={0.15}>
              <dt>Asks instead of guessing</dt>
              <dd>Anything unclear becomes a question in Needs you, not a wrong implementation.</dd>
            </Reveal>
            <Reveal delay={0.20}>
              <dt>Logs who did what</dt>
              <dd>Every change records whether you or the agent made it. A <span className="dot">●</span> marks agent work you haven't seen.</dd>
            </Reveal>
            <Reveal delay={0.25}>
              <dt>Keeps your data local</dt>
              <dd>One SQLite file on your machine. The CLI holds no credentials and never calls Jira itself.</dd>
            </Reveal>
          </dl>
        </section>

        <section id="install" className="section install" aria-labelledby="install-title">
          <Reveal>
            <h2 id="install-title">Try it tonight</h2>
            <p className="section-lede">Three steps, about two minutes.</p>
          </Reveal>
          <ol className="install-steps">
            <Reveal as="li" delay={0.00}>
              <h3>Install the CLI</h3>
              <p>
                One small binary. You don't need Go or anything else installed. On macOS, Homebrew asks you to
                trust a new tap once before installing from it.
              </p>
              <Command text={`brew tap ${OWNER}/tap`} />
              <Command text={`brew trust --cask ${OWNER}/tap/tars`} />
              <Command text="brew install --cask tars" />
              <p className="alt">
                Check it worked with <code>tars --version</code>. On Linux or Windows, download a binary from{" "}
                <a href={`https://github.com/${REPO}/releases/latest`}>GitHub Releases</a>, or build it with{" "}
                <code>go install github.com/{REPO}/cmd/tars@latest</code>.
              </p>
            </Reveal>
            <Reveal as="li" delay={0.08}>
              <h3>Add the plugin to Claude Code</h3>
              <p>
                Run these inside Claude Code. Later, <code>/plugin marketplace update tars</code> gets you new
                versions.
              </p>
              <Command text={`/plugin marketplace add ${REPO}`} />
              <Command text="/plugin install tars@tars" />
            </Reveal>
            <Reveal as="li" delay={0.16}>
              <h3>Tell it what's on your plate</h3>
              <p>
                Then run <code>tars</code> in any terminal to see your list.
              </p>
              <Command text="/tars:tars fix the flaky login test by thursday" />
            </Reveal>
          </ol>
        </section>

        <section className="closing" aria-labelledby="closing-title">
          <Reveal className="closing-inner">
            <Mark size={40} />
            <h2 id="closing-title">Hand the bookkeeping to your agent</h2>
            <div className="ctas">
              <motion.a className="btn btn-primary" href="#install" {...press}>
                <PromptIcon />
                Install Tars
              </motion.a>
              <motion.a className="btn btn-quiet" href={`https://github.com/${REPO}`} {...press}>
                <GitHubIcon />
                View on GitHub
              </motion.a>
            </div>
          </Reveal>
        </section>
      </main>

      <footer className="footer">
        <p>Tars is free and open source.</p>
        <a href={`https://github.com/${REPO}`}>GitHub</a>
      </footer>
    </>
  );
}

function Command({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      // Clipboard can be blocked (e.g. insecure context); the text stays selectable.
    }
  };
  return (
    <div className="cmd">
      <code>{text}</code>
      <motion.button
        type="button"
        whileTap={{ scale: 0.88 }}
        onClick={copy}
        className={copied ? "copied" : undefined}
        aria-label={copied ? "Copied" : `Copy: ${text}`}
        title={copied ? "Copied" : "Copy"}
      >
        <AnimatePresence mode="popLayout" initial={false}>
          <motion.span
            key={copied ? "check" : "copy"}
            className="cmd-icon"
            initial={{ opacity: 0, scale: 0.6, rotate: copied ? -30 : 0 }}
            animate={{ opacity: 1, scale: 1, rotate: 0 }}
            exit={{ opacity: 0, scale: 0.6 }}
            transition={{ type: "spring", stiffness: 500, damping: 28 }}
          >
            {copied ? <CheckIcon /> : <CopyIcon />}
          </motion.span>
        </AnimatePresence>
      </motion.button>
    </div>
  );
}

function CopyIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="5.25" y="5.25" width="8.5" height="8.5" rx="2" />
      <path d="M10.75 5.25V4a1.75 1.75 0 0 0-1.75-1.75H4A1.75 1.75 0 0 0 2.25 4v5c0 .97.78 1.75 1.75 1.75h1.25" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3.5 8.5l3 3 6-7" />
    </svg>
  );
}

function PromptIcon() {
  return (
    <svg className="btn-icon" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 4.5 6.5 8 3 11.5" />
      <path d="M8.5 11.5H13" />
    </svg>
  );
}

function ChevronDownIcon() {
  return (
    <svg className="btn-icon btn-icon-down" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M4 6.5 8 10.5 12 6.5" />
    </svg>
  );
}

function GitHubIcon() {
  return (
    <svg className="btn-icon" width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <path d="M8 .2a8 8 0 0 0-2.53 15.59c.4.07.55-.17.55-.38v-1.33c-2.23.48-2.7-1.07-2.7-1.07-.36-.92-.89-1.17-.89-1.17-.73-.5.06-.49.06-.49.8.06 1.23.83 1.23.83.71 1.23 1.88.87 2.34.66.07-.52.28-.87.5-1.07-1.78-.2-3.65-.89-3.65-3.96 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.6 7.6 0 0 1 4 0c1.53-1.03 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.28.82 2.15 0 3.08-1.87 3.75-3.66 3.95.29.25.54.74.54 1.49v2.2c0 .21.15.46.55.38A8 8 0 0 0 8 .2Z" />
    </svg>
  );
}
