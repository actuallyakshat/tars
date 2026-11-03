import { useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Capture } from "./Capture";
import { Control, Matching, Profiles, Reports, Surfaces, Threads, Unfiled } from "./Story";
import { Mark } from "./Mark";
import { Reveal, press } from "./Reveal";

// Update when the repo is published under its final GitHub owner.
const REPO = "actuallyakshat/tars";
const OWNER = REPO.split("/")[0];

// The hero's load-in: one short staggered sequence, then the capture diagram plays.
const heroItem = {
  hidden: { opacity: 0, y: 10 },
  shown: { opacity: 1, y: 0, transition: { duration: 0.6, ease: [0.2, 0.7, 0.2, 1] } },
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
          <a href="#how">How it works</a>
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
            <motion.h1 variants={heroItem}>Every Claude Code session, recorded and sorted</motion.h1>
            <motion.p className="lede" variants={heroItem}>
              Install the plugin and Tars records each session you run in a git repo as it happens, and links it to
              the task it was for. Agents leave a few lines on what they did. You get one place to see everything
              your agents have done.
            </motion.p>
            <motion.div className="ctas" variants={heroItem}>
              <motion.a className="btn btn-primary" href="#install" {...press}>
                <PromptIcon />
                Install Tars
              </motion.a>
              <motion.a className="btn btn-quiet" href="#how" {...press}>
                How capture works
              </motion.a>
            </motion.div>
          </div>
          <motion.div className="hero-visual" variants={heroItem}>
            <Capture />
          </motion.div>
        </motion.section>

        <Matching />
        <Threads />
        <Unfiled />
        <Reports />
        <Surfaces />
        <Profiles />
        <Control />

        <section id="install" className="section install" aria-labelledby="install-title">
          <Reveal>
            <h2 id="install-title">Install</h2>
            <p className="section-lede">The CLI and the plugin are all you need. The Mac app is optional.</p>
          </Reveal>
          <ol className="install-steps">
            <Reveal as="li" delay={0.0}>
              <h3>Install the CLI</h3>
              <p>
                One small binary. You don&rsquo;t need Go or anything else installed. Homebrew asks you to trust a
                new tap once before installing from it.
              </p>
              <Command text={`brew tap ${OWNER}/tap`} />
              <Command text={`brew trust --cask ${OWNER}/tap/tars`} />
              <Command text="brew install --cask tars" />
              <p className="alt">
                Check it worked with <code>tars --version</code>. On Linux or Windows, download a binary from{" "}
                <a href={`https://github.com/${REPO}/releases/latest`}>GitHub Releases</a>.
              </p>
            </Reveal>
            <Reveal as="li" delay={0.06}>
              <h3>Add the plugin to Claude Code</h3>
              <p>Run these inside Claude Code. Capture starts with your next session.</p>
              <Command text={`/plugin marketplace add ${REPO}`} />
              <Command text="/plugin install tars@tars" />
            </Reveal>
            <Reveal as="li" delay={0.12}>
              <h3>Add the Mac app, if you want it</h3>
              <p>
                It reads the same file as the CLI. Later, <code>/tars:update</code> upgrades the CLI, the plugin
                and the app together.
              </p>
              <Command text={`brew install --cask ${OWNER}/tap/tars-app`} />
            </Reveal>
            <Reveal as="li" delay={0.18}>
              <h3>Work as you normally do</h3>
              <p>
                Open Claude Code in any git repo and start a session. Afterwards, run <code>tars</code> to see where
                it was filed.
              </p>
            </Reveal>
          </ol>
        </section>

        <section className="closing" aria-labelledby="closing-title">
          <Reveal className="closing-inner">
            <Mark size={40} />
            <h2 id="closing-title">Your agents do the work. Tars keeps the record.</h2>
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

function GitHubIcon() {
  return (
    <svg className="btn-icon" width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <path d="M8 .2a8 8 0 0 0-2.53 15.59c.4.07.55-.17.55-.38v-1.33c-2.23.48-2.7-1.07-2.7-1.07-.36-.92-.89-1.17-.89-1.17-.73-.5.06-.49.06-.49.8.06 1.23.83 1.23.83.71 1.23 1.88.87 2.34.66.07-.52.28-.87.5-1.07-1.78-.2-3.65-.89-3.65-3.96 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.6 7.6 0 0 1 4 0c1.53-1.03 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.28.82 2.15 0 3.08-1.87 3.75-3.66 3.95.29.25.54.74.54 1.49v2.2c0 .21.15.46.55.38A8 8 0 0 0 8 .2Z" />
    </svg>
  );
}
