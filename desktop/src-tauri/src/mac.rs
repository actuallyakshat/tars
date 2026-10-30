//! The few macOS integrations the app needs: clipboard, Finder, the browser
//! and Terminal.

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

/// AppleScript that opens a new Terminal window running the resume command,
/// from the folder the session started in when there is one.
pub fn terminal_script(id: &str, launch_dir: Option<&str>) -> Result<String> {
    let mut cmd = resume_command(id)?;
    if let Some(dir) = launch_dir.map(str::trim).filter(|d| !d.is_empty()) {
        cmd = format!("cd {} && {cmd}", shell_quote(dir));
    }
    Ok(format!(
        "tell application \"Terminal\"\n  activate\n  do script {}\nend tell",
        applescript_string(&cmd)
    ))
}

pub fn open_in_terminal(id: &str, launch_dir: Option<&str>) -> Result<()> {
    let script = terminal_script(id, launch_dir)?;
    run(Command::new("osascript").arg("-e").arg(script), "open Terminal")
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
        let s = terminal_script("sess-42", Some("/work/it's \"here\"")).unwrap();
        assert_eq!(
            s,
            "tell application \"Terminal\"\n  activate\n  do script \"cd '/work/it'\\\\''s \\\"here\\\"' && claude --resume sess-42\"\nend tell"
        );
        let s = terminal_script("sess-42", None).unwrap();
        assert!(s.contains("do script \"claude --resume sess-42\""));
        assert!(terminal_script("bad id", None).is_err());
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
