//! The few macOS integrations the app needs: clipboard, Finder, the browser
//! and the terminal (Terminal.app, iTerm2 or Ghostty).

use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

use crate::cli::{expand_home, CliError, ErrorKind, Result};

fn sys(msg: impl Into<String>) -> CliError {
    CliError::new(ErrorKind::System, msg)
}

/// Claude session IDs are UUIDs; anything else is refused before it gets near
/// a shell or AppleScript.
pub fn valid_session_id(id: &str) -> bool {
    !id.is_empty() && id.len() <= 128 && id.chars().all(|c| c.is_ascii_alphanumeric() || c == '-' || c == '_')
}

pub fn resume_command(id: &str) -> Result<String> {
    if !valid_session_id(id) {
        return Err(sys(format!("{id:?} doesn't look like a Claude session ID.")));
    }
    Ok(format!("claude --resume {id}"))
}

fn shell_quote(s: &str) -> String {
    format!("'{}'", s.replace('\'', r"'\''"))
}

fn applescript_string(s: &str) -> String {
    format!("\"{}\"", s.replace('\\', r"\\").replace('"', "\\\""))
}

/// The terminal apps Resume in Terminal knows how to drive.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TerminalApp {
    Terminal,
    Iterm,
    Ghostty,
}

impl TerminalApp {
    fn bundle(self) -> &'static str {
        match self {
            TerminalApp::Terminal => "Terminal.app",
            TerminalApp::Iterm => "iTerm.app",
            TerminalApp::Ghostty => "Ghostty.app",
        }
    }

    fn from_term_program(v: &str) -> Option<Self> {
        match v {
            "Apple_Terminal" => Some(TerminalApp::Terminal),
            "iTerm.app" => Some(TerminalApp::Iterm),
            "ghostty" => Some(TerminalApp::Ghostty),
            _ => None,
        }
    }
}

/// The terminal tars was launched from when we know it ($TERM_PROGRAM), else
/// the first installed of Ghostty and iTerm2, else Terminal.app. Opening the
/// app from the Dock leaves $TERM_PROGRAM unset, hence the fallback.
pub fn pick_terminal(term_program: Option<&str>, installed: impl Fn(TerminalApp) -> bool) -> TerminalApp {
    term_program
        .and_then(TerminalApp::from_term_program)
        .or_else(|| [TerminalApp::Ghostty, TerminalApp::Iterm].into_iter().find(|&t| installed(t)))
        .unwrap_or(TerminalApp::Terminal)
}

fn is_installed(app: TerminalApp) -> bool {
    let home = std::env::var_os("HOME").map(PathBuf::from);
    [Some(PathBuf::from("/Applications")), home.map(|h| h.join("Applications"))]
        .into_iter()
        .flatten()
        .any(|dir| dir.join(app.bundle()).exists())
}

fn launch_dir(dir: Option<&str>) -> Option<&str> {
    dir.map(str::trim).filter(|d| !d.is_empty())
}

/// AppleScript that opens a new Terminal or iTerm2 window running the resume
/// command, from the folder the session started in when there is one.
pub fn terminal_script(app: TerminalApp, id: &str, dir: Option<&str>) -> Result<String> {
    let mut cmd = resume_command(id)?;
    if let Some(dir) = launch_dir(dir) {
        cmd = format!("cd {} && {cmd}", shell_quote(dir));
    }
    let cmd = applescript_string(&cmd);
    Ok(match app {
        TerminalApp::Iterm => format!(
            "tell application \"iTerm\"\n  activate\n  set w to (create window with default profile)\n  tell current session of w to write text {cmd}\nend tell"
        ),
        _ => format!("tell application \"Terminal\"\n  activate\n  do script {cmd}\nend tell"),
    })
}

/// Arguments for `open` that start a Ghostty window in the session's folder
/// and type the resume command into its shell, so the window outlives claude.
/// Ghostty 1.2 has no AppleScript; `input` arrived in 1.2.0.
pub fn ghostty_args(id: &str, dir: Option<&str>) -> Result<Vec<String>> {
    let cmd = resume_command(id)?;
    let mut args: Vec<String> = ["-na", "Ghostty.app", "--args"].map(String::from).into();
    if let Some(dir) = launch_dir(dir) {
        args.push(format!("--working-directory={dir}"));
    }
    args.push(format!("--input=raw:{cmd}\\n"));
    Ok(args)
}

pub fn open_in_terminal(id: &str, launch_dir: Option<&str>) -> Result<()> {
    let term_program = std::env::var("TERM_PROGRAM").ok();
    match pick_terminal(term_program.as_deref(), is_installed) {
        TerminalApp::Ghostty => run(Command::new("open").args(ghostty_args(id, launch_dir)?), "open Ghostty"),
        app => {
            let script = terminal_script(app, id, launch_dir)?;
            let what = if app == TerminalApp::Iterm { "open iTerm2" } else { "open Terminal" };
            run(Command::new("osascript").arg("-e").arg(script), what)
        }
    }
}

pub fn copy_text(text: &str) -> Result<()> {
    let mut child = Command::new("pbcopy")
        .stdin(Stdio::piped())
        .spawn()
        .map_err(|e| sys(format!("Couldn't copy: {e}")))?;
    child
        .stdin
        .take()
        .expect("piped stdin")
        .write_all(text.as_bytes())
        .map_err(|e| sys(format!("Couldn't copy: {e}")))?;
    let status = child.wait().map_err(|e| sys(format!("Couldn't copy: {e}")))?;
    if !status.success() {
        return Err(sys("Couldn't copy to the clipboard."));
    }
    Ok(())
}

/// The path to reveal, if it's absolute (after ~) and exists.
pub fn reveal_target(path: &str, home: Option<&Path>) -> Result<PathBuf> {
    let p = expand_home(path.trim(), home);
    if !p.is_absolute() {
        return Err(sys(format!("{path} isn't a full path, so Finder can't show it.")));
    }
    if !p.exists() {
        return Err(sys(format!("{} doesn't exist on this Mac.", p.display())));
    }
    Ok(p)
}

pub fn reveal(path: &str, home: Option<&Path>) -> Result<()> {
    let p = reveal_target(path, home)?;
    run(Command::new("open").arg("-R").arg(p), "reveal in Finder")
}

pub fn valid_url(url: &str) -> bool {
    (url.starts_with("https://") || url.starts_with("http://")) && !url.chars().any(char::is_whitespace)
}

pub fn open_url(url: &str) -> Result<()> {
    if !valid_url(url) {
        return Err(sys(format!("{url} isn't a web address.")));
    }
    run(Command::new("open").arg(url), "open the link")
}

/// The folder picker. None when the user cancels.
pub fn choose_folder() -> Result<Option<String>> {
    let out = Command::new("osascript")
        .arg("-e")
        .arg("POSIX path of (choose folder with prompt \"Choose the repo's folder\")")
        .stdin(Stdio::null())
        .output()
        .map_err(|e| sys(format!("Couldn't open the folder picker: {e}")))?;
    if !out.status.success() {
        let err = String::from_utf8_lossy(&out.stderr);
        if err.contains("-128") {
            return Ok(None);
        }
        return Err(sys(format!("Couldn't open the folder picker: {}", err.trim())));
    }
    Ok(Some(folder_path(&String::from_utf8_lossy(&out.stdout))))
}

fn folder_path(s: &str) -> String {
    let p = s.trim();
    if p.len() > 1 { p.trim_end_matches('/').to_string() } else { p.to_string() }
}

fn run(cmd: &mut Command, what: &str) -> Result<()> {
    let out = cmd.stdin(Stdio::null()).output().map_err(|e| sys(format!("Couldn't {what}: {e}")))?;
    if !out.status.success() {
        let err = String::from_utf8_lossy(&out.stderr);
        return Err(sys(format!("Couldn't {what}: {}", err.trim())));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn folder_paths_lose_the_trailing_slash() {
        assert_eq!(folder_path("/Users/me/code/billing/\n"), "/Users/me/code/billing");
        assert_eq!(folder_path("/\n"), "/");
    }

    #[test]
    fn session_ids_are_validated() {
        assert!(valid_session_id("0b6a3c1e-7f2d-4b8e-9a51-2c4d6e8f0a1b"));
        assert!(valid_session_id("sess_42"));
        assert!(!valid_session_id(""));
        assert!(!valid_session_id("x; rm -rf ~"));
        assert!(!valid_session_id("a\"b"));
        assert_eq!(resume_command("sess-42").unwrap(), "claude --resume sess-42");
        assert!(resume_command("$(id)").is_err());
    }

    #[test]
    fn terminal_script_quotes_the_folder() {
        let s = terminal_script(TerminalApp::Terminal, "sess-42", Some("/work/it's \"here\"")).unwrap();
        assert_eq!(
            s,
            "tell application \"Terminal\"\n  activate\n  do script \"cd '/work/it'\\\\''s \\\"here\\\"' && claude --resume sess-42\"\nend tell"
        );
        let s = terminal_script(TerminalApp::Terminal, "sess-42", None).unwrap();
        assert!(s.contains("do script \"claude --resume sess-42\""));
        assert!(terminal_script(TerminalApp::Terminal, "bad id", None).is_err());
    }

    #[test]
    fn iterm_script_writes_into_a_new_window() {
        let s = terminal_script(TerminalApp::Iterm, "sess-42", Some("/work/billing")).unwrap();
        assert!(s.starts_with("tell application \"iTerm\""));
        assert!(s.contains("create window with default profile"));
        assert!(s.contains("write text \"cd '/work/billing' && claude --resume sess-42\""));
    }

    #[test]
    fn ghostty_gets_the_folder_and_the_command_as_input() {
        assert_eq!(
            ghostty_args("sess-42", Some(" /work/my billing ")).unwrap(),
            ["-na", "Ghostty.app", "--args", "--working-directory=/work/my billing", "--input=raw:claude --resume sess-42\\n"]
        );
        assert_eq!(ghostty_args("sess-42", Some("")).unwrap().len(), 4);
        assert!(ghostty_args("$(id)", None).is_err());
    }

    #[test]
    fn terminal_follows_term_program_then_what_is_installed() {
        let none = |_: TerminalApp| false;
        let all = |_: TerminalApp| true;
        assert_eq!(pick_terminal(Some("ghostty"), none), TerminalApp::Ghostty);
        assert_eq!(pick_terminal(Some("iTerm.app"), all), TerminalApp::Iterm);
        assert_eq!(pick_terminal(Some("Apple_Terminal"), all), TerminalApp::Terminal);
        assert_eq!(pick_terminal(None, all), TerminalApp::Ghostty);
        assert_eq!(pick_terminal(Some("vscode"), |t| t == TerminalApp::Iterm), TerminalApp::Iterm);
        assert_eq!(pick_terminal(None, none), TerminalApp::Terminal);
    }

    #[test]
    fn reveal_needs_an_existing_absolute_path() {
        let dir = tempfile::tempdir().unwrap();
        let p = dir.path().to_str().unwrap();
        assert_eq!(reveal_target(p, None).unwrap(), dir.path());
        assert!(reveal_target("../billing-12", None).is_err());
        assert!(reveal_target("/definitely/not/here", None).is_err());
        let home = dir.path().parent().unwrap();
        let name = dir.path().file_name().unwrap().to_str().unwrap();
        assert_eq!(reveal_target(&format!("~/{name}"), Some(home)).unwrap(), dir.path());
    }

    #[test]
    fn only_web_urls_open() {
        assert!(valid_url("https://acme.atlassian.net/browse/BILL-881"));
        assert!(!valid_url("file:///etc/passwd"));
        assert!(!valid_url("https://x y"));
        assert!(open_url("javascript:alert(1)").is_err());
    }
}
