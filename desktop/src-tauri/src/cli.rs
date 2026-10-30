//! Runs the tars CLI. Every read and write goes through `tars --json …`; the
//! app never opens the database itself.

use std::ffi::OsString;
use std::io::ErrorKind as IoErrorKind;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

use serde::{Deserialize, Serialize};
use serde_json::Value;

/// Folders GUI apps on macOS don't get on PATH but where tars usually lives.
pub const EXTRA_DIRS: &[&str] = &["/opt/homebrew/bin", "/usr/local/bin"];

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ErrorKind {
    /// No tars binary found.
    Missing,
    /// A tars binary that predates reports and sessions.
    TooOld,
    /// The command ran and failed, e.g. "#4 has no open question".
    Failed,
    /// The command succeeded but printed something that isn't the expected JSON.
    BadOutput,
    /// Anything outside the CLI: clipboard, Terminal, Finder.
    System,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct CliError {
    pub kind: ErrorKind,
    pub message: String,
}

impl CliError {
    pub fn new(kind: ErrorKind, message: impl Into<String>) -> Self {
        CliError { kind, message: message.into() }
    }
}

impl std::fmt::Display for CliError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:?}: {}", self.kind, self.message)
    }
}

impl std::error::Error for CliError {}

pub type Result<T> = std::result::Result<T, CliError>;

/// Candidate tars binaries in the order they should be tried. `TARS_BIN`, when
/// set, is the only candidate. Otherwise PATH comes first, then the Homebrew
/// folders, then the Go bin directory.
pub fn candidates(
    tars_bin: Option<&str>,
    path: Option<&str>,
    home: Option<&Path>,
    exists: impl Fn(&Path) -> bool,
) -> Vec<PathBuf> {
    if let Some(bin) = tars_bin.map(str::trim).filter(|b| !b.is_empty()) {
        let p = expand_home(bin, home);
        if p.components().count() > 1 {
            return if exists(&p) { vec![p] } else { vec![] };
        }
        return dirs(path, home)
            .into_iter()
            .map(|d| d.join(bin))
            .filter(|p| exists(p))
            .take(1)
            .collect();
    }
    let mut out: Vec<PathBuf> = Vec::new();
    for d in dirs(path, home) {
        let p = d.join("tars");
        if exists(&p) && !out.contains(&p) {
            out.push(p);
        }
    }
    out
}

fn dirs(path: Option<&str>, home: Option<&Path>) -> Vec<PathBuf> {
    let mut out: Vec<PathBuf> = path
        .unwrap_or("")
        .split(':')
        .filter(|d| !d.is_empty())
        .map(PathBuf::from)
        .collect();
    out.extend(EXTRA_DIRS.iter().map(PathBuf::from));
    if let Some(h) = home {
        out.push(h.join("go").join("bin"));
    }
    out
}

pub fn expand_home(p: &str, home: Option<&Path>) -> PathBuf {
    match (p.strip_prefix("~/"), home) {
        (Some(rest), Some(h)) => h.join(rest),
        _ if p == "~" && home.is_some() => home.unwrap().to_path_buf(),
        _ => PathBuf::from(p),
    }
}

/// What the UI sends to create a task. Empty fields are left out.
#[derive(Debug, Default, Clone, Deserialize)]
#[serde(default)]
pub struct NewTask {
    pub title: String,
    pub priority: String,
    pub jira: String,
    pub repo: String,
    pub due: String,
    pub tags: String,
    pub body: String,
}

/// Arguments after the global flags for each command the app uses. Free text
/// goes after `--` so a note starting with "-" isn't read as a flag.
pub mod args {
    use super::NewTask;

    pub fn probe() -> Vec<String> {
        vec!["ls".into(), "--needs-you".into()]
    }

    pub fn list() -> Vec<String> {
        vec!["ls".into(), "--all".into()]
    }

    pub fn show(id: i64) -> Vec<String> {
        vec!["show".into(), id.to_string()]
    }

    pub fn mv(id: i64, status: &str) -> Vec<String> {
        vec!["mv".into(), id.to_string(), status.into()]
    }

    pub fn done(id: i64) -> Vec<String> {
        vec!["done".into(), id.to_string()]
    }

    pub fn answer(id: i64, text: &str) -> Vec<String> {
        vec!["answer".into(), id.to_string(), "--".into(), text.into()]
    }

    pub fn note(id: i64, text: &str) -> Vec<String> {
        vec!["note".into(), id.to_string(), "--".into(), text.into()]
    }

    pub fn seen(id: i64) -> Vec<String> {
        vec!["seen".into(), id.to_string()]
    }

    pub fn add(t: &NewTask) -> Vec<String> {
        let mut a = vec!["add".to_string()];
        for (flag, v) in [
            ("--priority", &t.priority),
            ("--jira", &t.jira),
            ("--repo", &t.repo),
            ("--due", &t.due),
            ("--tag", &t.tags),
            ("--body", &t.body),
        ] {
            let v = v.trim();
            if !v.is_empty() {
                a.push(flag.into());
                a.push(v.into());
            }
        }
        a.push("--".into());
        a.push(t.title.trim().into());
        a
    }
}

#[derive(Debug, Clone)]
pub struct Tars {
    pub bin: PathBuf,
    /// Passed as --db when set; otherwise the CLI uses $TARS_HOME or its default.
    pub db: Option<PathBuf>,
    /// Extra environment for every command, e.g. TARS_HOME in tests.
    pub envs: Vec<(OsString, OsString)>,
}

impl Tars {
    pub fn new(bin: impl Into<PathBuf>) -> Self {
        Tars { bin: bin.into(), db: None, envs: Vec::new() }
    }

    /// Full argv: global flags first, then the command.
    pub fn argv(&self, cmd: &[String]) -> Vec<OsString> {
        let mut out: Vec<OsString> = vec!["--json".into()];
        if let Some(db) = &self.db {
            out.push("--db".into());
            out.push(db.into());
        }
        out.extend(cmd.iter().map(OsString::from));
        out
    }

    /// Runs one command and returns its stdout.
    pub fn run(&self, cmd: &[String]) -> Result<String> {
        let mut c = Command::new(&self.bin);
        c.args(self.argv(cmd))
            .stdin(Stdio::null())
            // Changes made from the app are the user's own, never a Claude
            // session's, even when the app was started from inside one.
            .env_remove("CLAUDE_CODE_SESSION_ID");
        for (k, v) in &self.envs {
            c.env(k, v);
        }
        let out = c.output().map_err(|e| spawn_error(&self.bin, e))?;
        if !out.status.success() {
            let stderr = String::from_utf8_lossy(&out.stderr);
            let stdout = String::from_utf8_lossy(&out.stdout);
            let text = if stderr.trim().is_empty() { stdout } else { stderr };
            return Err(classify_failure(&self.bin, &text));
        }
        Ok(String::from_utf8_lossy(&out.stdout).into_owned())
    }

    pub fn version(&self) -> Result<String> {
        let mut c = Command::new(&self.bin);
        c.arg("--version").stdin(Stdio::null());
        for (k, v) in &self.envs {
            c.env(k, v);
        }
        let out = c.output().map_err(|e| spawn_error(&self.bin, e))?;
        Ok(parse_version(&String::from_utf8_lossy(&out.stdout)))
    }

    /// Checks this binary understands what the app needs.
    pub fn probe(&self) -> Result<()> {
        parse_list(&self.run(&args::probe())?).map(|_| ())
    }

    pub fn list(&self) -> Result<Value> {
        parse_list(&self.run(&args::list())?)
    }

    pub fn show(&self, id: i64) -> Result<Value> {
        parse_task(&self.run(&args::show(id))?)
    }

    pub fn add(&self, t: &NewTask) -> Result<Value> {
        if t.title.trim().is_empty() {
            return Err(CliError::new(ErrorKind::Failed, "A task needs a title."));
        }
        parse_task(&self.run(&args::add(t))?)
    }

    pub fn mv(&self, id: i64, status: &str) -> Result<Value> {
        parse_list(&self.run(&args::mv(id, status))?)
    }

    pub fn done(&self, id: i64) -> Result<Value> {
        parse_list(&self.run(&args::done(id))?)
    }

    pub fn answer(&self, id: i64, text: &str) -> Result<Value> {
        parse_object(&self.run(&args::answer(id, text))?)
    }

    pub fn note(&self, id: i64, text: &str) -> Result<Value> {
        parse_object(&self.run(&args::note(id, text))?)
    }

    /// Clears the ● for agent updates once the user has looked at the task.
    pub fn seen(&self, id: i64) -> Result<Value> {
        parse_object(&self.run(&args::seen(id))?)
    }
}

/// Tries each candidate and returns the first one new enough. If every one is
/// too old, the error names the first.
pub fn detect(found: Vec<PathBuf>, configure: impl Fn(&mut Tars)) -> Result<Tars> {
    if found.is_empty() {
        return Err(missing());
    }
    let mut first_err = None;
    for bin in found {
        let mut t = Tars::new(bin);
        configure(&mut t);
        match t.probe() {
            Ok(()) => return Ok(t),
            Err(e) if first_err.is_none() => first_err = Some(e),
            Err(_) => {}
        }
    }
    Err(first_err.unwrap())
}

pub fn missing() -> CliError {
    CliError::new(
        ErrorKind::Missing,
        "Couldn't find the tars CLI. Install it with `brew install --cask tars` or \
         `go install github.com/actuallyakshat/tars/cmd/tars@latest`, or set TARS_BIN \
         to its full path.",
    )
}

fn spawn_error(bin: &Path, e: std::io::Error) -> CliError {
    if e.kind() == IoErrorKind::NotFound {
        return missing();
    }
    CliError::new(ErrorKind::System, format!("Couldn't run {}: {e}", bin.display()))
}

/// Maps a failed command's output to an error the UI can show as is.
pub fn classify_failure(bin: &Path, output: &str) -> CliError {
    let msg = clean_message(output);
    let lower = msg.to_lowercase();
    if lower.contains("unknown command") || lower.contains("unknown flag") || lower.contains("unknown shorthand flag") {
        return CliError::new(
            ErrorKind::TooOld,
            format!(
                "{} is too old for this app: it has no agent reports or session links ({msg}). \
                 Upgrade with `brew upgrade --cask tars`, or build it from source with \
                 `go install ./cmd/tars` and set TARS_BIN to the new binary.",
                bin.display()
            ),
        );
    }
    CliError::new(ErrorKind::Failed, msg)
}

/// Strips colour codes and the CLI's "✘ " prefix.
pub fn clean_message(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    let mut chars = s.chars().peekable();
    while let Some(c) = chars.next() {
        if c == '\u{1b}' {
            if chars.peek() == Some(&'[') {
                chars.next();
                for c in chars.by_ref() {
                    if c.is_ascii_alphabetic() {
                        break;
                    }
                }
            }
            continue;
        }
        out.push(c);
    }
    let t = out.trim();
    let t = t.strip_prefix("✘").unwrap_or(t).trim_start();
    let t = t.strip_prefix("Error:").unwrap_or(t).trim();
    t.lines().next().unwrap_or("").trim().to_string()
}

pub fn parse_version(s: &str) -> String {
    let s = s.trim();
    s.rsplit(' ').next().unwrap_or(s).to_string()
}

fn parse_json(s: &str) -> Result<Value> {
    serde_json::from_str(s).map_err(|e| {
        CliError::new(ErrorKind::BadOutput, format!("tars printed something that isn't JSON ({e})."))
    })
}

pub fn parse_list(s: &str) -> Result<Value> {
    let v = parse_json(s)?;
    match &v {
        Value::Array(items) if items.iter().all(|t| t.get("id").is_some()) => Ok(v),
        _ => Err(CliError::new(ErrorKind::BadOutput, "Expected a list of tasks from tars.")),
    }
}

pub fn parse_task(s: &str) -> Result<Value> {
    let v = parse_json(s)?;
    if v.get("id").and_then(Value::as_i64).is_some() && v.get("title").is_some() {
        Ok(v)
    } else {
        Err(CliError::new(ErrorKind::BadOutput, "Expected a task from tars."))
    }
}

fn parse_object(s: &str) -> Result<Value> {
    let v = parse_json(s)?;
    if v.is_object() {
        Ok(v)
    } else {
        Err(CliError::new(ErrorKind::BadOutput, "Unexpected output from tars."))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashSet;

    fn fs(paths: &[&str]) -> impl Fn(&Path) -> bool {
        let set: HashSet<PathBuf> = paths.iter().map(PathBuf::from).collect();
        move |p: &Path| set.contains(p)
    }

    #[test]
    fn candidates_follow_path_then_homebrew_then_go() {
        let home = Path::new("/Users/me");
        let found = candidates(
            None,
            Some("/usr/bin:/bin"),
            Some(home),
            fs(&["/Users/me/go/bin/tars", "/opt/homebrew/bin/tars", "/bin/tars"]),
        );
        let want: Vec<PathBuf> =
            ["/bin/tars", "/opt/homebrew/bin/tars", "/Users/me/go/bin/tars"].iter().map(PathBuf::from).collect();
        assert_eq!(found, want);
    }

    #[test]
    fn candidates_dedupe_and_handle_empty_path() {
        let found = candidates(None, Some("/opt/homebrew/bin::"), None, fs(&["/opt/homebrew/bin/tars"]));
        assert_eq!(found, vec![PathBuf::from("/opt/homebrew/bin/tars")]);
        assert!(candidates(None, None, None, fs(&[])).is_empty());
    }

    #[test]
    fn tars_bin_wins_and_is_the_only_candidate() {
        let home = Path::new("/Users/me");
        let exists = fs(&["/opt/homebrew/bin/tars", "/Users/me/dev/tars", "/opt/homebrew/bin/tars-dev"]);
        assert_eq!(
            candidates(Some("~/dev/tars"), Some("/usr/bin"), Some(home), &exists),
            vec![PathBuf::from("/Users/me/dev/tars")]
        );
        // A bare name is looked up like a command.
        assert_eq!(
            candidates(Some("tars-dev"), Some("/usr/bin"), Some(home), &exists),
            vec![PathBuf::from("/opt/homebrew/bin/tars-dev")]
        );
        // Pointing at something that isn't there finds nothing, not a fallback.
        assert!(candidates(Some("/nope/tars"), Some("/usr/bin"), Some(home), &exists).is_empty());
        // Blank means unset.
        assert_eq!(candidates(Some("  "), None, None, &exists), vec![PathBuf::from("/opt/homebrew/bin/tars")]);
    }

    #[test]
    fn argv_puts_global_flags_first_and_text_after_dashes() {
        let mut t = Tars::new("/bin/tars");
        assert_eq!(t.argv(&args::list()), ["--json", "ls", "--all"].map(OsString::from));
        t.db = Some(PathBuf::from("/tmp/x/tars.db"));
        assert_eq!(
            t.argv(&args::note(4, "-v flag breaks it")),
            ["--json", "--db", "/tmp/x/tars.db", "note", "4", "--", "-v flag breaks it"].map(OsString::from)
        );
        assert_eq!(args::answer(7, "billing"), ["answer", "7", "--", "billing"]);
        assert_eq!(args::seen(9), ["seen", "9"]);
        assert_eq!(args::mv(3, "in-review"), ["mv", "3", "in-review"]);
        assert_eq!(args::done(3), ["done", "3"]);
        assert_eq!(args::show(12), ["show", "12"]);
        assert_eq!(args::probe(), ["ls", "--needs-you"]);
    }

    #[test]
    fn add_args_skip_empty_fields() {
        let t = NewTask {
            title: "  Fix export ".into(),
            priority: "high".into(),
            jira: "BILL-881".into(),
            tags: "perf, backend".into(),
            due: " ".into(),
            ..Default::default()
        };
        assert_eq!(
            args::add(&t),
            ["add", "--priority", "high", "--jira", "BILL-881", "--tag", "perf, backend", "--", "Fix export"]
        );
        let plain = NewTask { title: "x".into(), ..Default::default() };
        assert_eq!(args::add(&plain), ["add", "--", "x"]);
    }

    #[test]
    fn parses_fixtures() {
        let list = parse_list(include_str!("../tests/fixtures/ls.json")).unwrap();
        let items = list.as_array().unwrap();
        assert_eq!(items.len(), 3);
        assert_eq!(items[0]["priority"], 0);
        assert_eq!(items[1]["needs_you"], true);
        assert_eq!(items[2]["unread"], true);

        let task = parse_task(include_str!("../tests/fixtures/show.json")).unwrap();
        assert_eq!(task["report"]["needs_you"], "Alert #oncall or #billing-alerts?");
        assert_eq!(task["sessions"][0]["id"], "sess-42");
        assert_eq!(task["sessions"][0]["name"], "webhook retry");

        assert_eq!(parse_list("[]").unwrap(), Value::Array(vec![]));
    }

    #[test]
    fn rejects_unexpected_output() {
        assert_eq!(parse_list("Added #1 Fix").unwrap_err().kind, ErrorKind::BadOutput);
        assert_eq!(parse_list(r#"{"id":1}"#).unwrap_err().kind, ErrorKind::BadOutput);
        assert_eq!(parse_task("[]").unwrap_err().kind, ErrorKind::BadOutput);
        assert_eq!(parse_task(r#"{"deleted":[1]}"#).unwrap_err().kind, ErrorKind::BadOutput);
    }

    #[test]
    fn maps_old_cli_errors() {
        let bin = Path::new("/opt/homebrew/bin/tars");
        let e = classify_failure(bin, include_str!("../tests/fixtures/old-unknown-flag.txt"));
        assert_eq!(e.kind, ErrorKind::TooOld);
        assert!(e.message.contains("/opt/homebrew/bin/tars is too old"), "{}", e.message);
        assert!(e.message.contains("TARS_BIN"));
        let e = classify_failure(bin, include_str!("../tests/fixtures/old-unknown-command.txt"));
        assert_eq!(e.kind, ErrorKind::TooOld);
    }

    #[test]
    fn maps_command_failures_to_plain_messages() {
        let e = classify_failure(Path::new("tars"), "\u{1b}[1;31m✘ \u{1b}[0m#4 has no open question\n");
        assert_eq!(e, CliError::new(ErrorKind::Failed, "#4 has no open question"));
        assert_eq!(clean_message("Error: task 9: not found\n"), "task 9: not found");
    }

    #[test]
    fn missing_binary_is_reported_as_missing() {
        let e = Tars::new("/definitely/not/here/tars").list().unwrap_err();
        assert_eq!(e.kind, ErrorKind::Missing);
        assert!(e.message.contains("TARS_BIN"));
        assert_eq!(detect(vec![], |_| {}).unwrap_err().kind, ErrorKind::Missing);
    }

    #[test]
    fn add_requires_a_title() {
        let e = Tars::new("/bin/false").add(&NewTask::default()).unwrap_err();
        assert_eq!(e.kind, ErrorKind::Failed);
    }

    #[test]
    fn version_is_the_last_word() {
        assert_eq!(parse_version("tars version 0.1.3\n"), "0.1.3");
        assert_eq!(parse_version("tars version dev"), "dev");
    }

    #[test]
    fn expands_home() {
        let h = Path::new("/Users/me");
        assert_eq!(expand_home("~/go/bin/tars", Some(h)), PathBuf::from("/Users/me/go/bin/tars"));
        assert_eq!(expand_home("/abs", Some(h)), PathBuf::from("/abs"));
        assert_eq!(expand_home("~/x", None), PathBuf::from("~/x"));
    }
}
