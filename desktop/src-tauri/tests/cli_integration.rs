//! Builds the Go CLI from this checkout and drives it through the wrapper
//! against a throwaway database.

use std::ffi::OsString;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::OnceLock;

use serde_json::Value;
use tars_desktop::cli::{self, ErrorKind, NewTask, TaskEdit, Tars};

fn repo_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../..").canonicalize().unwrap()
}

/// The tars binary, built once per test run.
fn tars_bin() -> &'static Path {
    static BIN: OnceLock<(tempfile::TempDir, PathBuf)> = OnceLock::new();
    &BIN.get_or_init(|| {
        let dir = tempfile::tempdir().unwrap();
        let bin = dir.path().join("tars");
        let out = Command::new("go")
            .args(["build", "-o"])
            .arg(&bin)
            .arg("./cmd/tars")
            .current_dir(repo_root())
            .output()
            .expect("go must be installed to run this test");
        assert!(out.status.success(), "go build failed: {}", String::from_utf8_lossy(&out.stderr));
        (dir, bin)
    })
    .1
}

struct Env {
    _dir: tempfile::TempDir,
    home: PathBuf,
    claude: PathBuf,
    tars: Tars,
}

/// A temp TARS_HOME and Claude config dir. CLAUDE_CODE_SESSION_ID is always
/// set explicitly so these tests never link the session running them.
fn env() -> Env {
    let dir = tempfile::tempdir().unwrap();
    let home = dir.path().join("tars-home");
    let claude = dir.path().join("claude");
    let proj = claude.join("projects").join("-work");
    std::fs::create_dir_all(&proj).unwrap();
    std::fs::write(
        proj.join("sess-42.jsonl"),
        "{\"type\":\"user\",\"cwd\":\"/work\"}\n{\"type\":\"custom-title\",\"customTitle\":\"webhook retry\"}\n",
    )
    .unwrap();
    let mut tars = Tars::new(tars_bin());
    tars.envs = vec![
        ("TARS_HOME".into(), home.clone().into_os_string()),
        ("CLAUDE_CONFIG_DIR".into(), claude.clone().into_os_string()),
        ("CLAUDE_CODE_SESSION_ID".into(), OsString::new()),
    ];
    Env { _dir: dir, home, claude, tars }
}

impl Env {
    /// Runs tars as an agent inside a Claude Code session would.
    fn as_agent(&self, session: &str, args: &[&str]) {
        let out = Command::new(tars_bin())
            .args(args)
            .env("TARS_HOME", &self.home)
            .env("CLAUDE_CONFIG_DIR", &self.claude)
            .env("CLAUDE_CODE_SESSION_ID", session)
            .output()
            .unwrap();
        assert!(out.status.success(), "tars {args:?}: {}", String::from_utf8_lossy(&out.stderr));
    }
}

fn ids(v: &Value) -> Vec<i64> {
    v.as_array().unwrap().iter().map(|t| t["id"].as_i64().unwrap()).collect()
}

#[test]
fn reads_and_writes_through_the_real_cli() {
    let e = env();
    let t = &e.tars;
    t.probe().unwrap();
    assert_eq!(t.list().unwrap(), Value::Array(vec![]));

    let added = t
        .add(&NewTask {
            title: "Add retry to webhook sender".into(),
            priority: "high".into(),
            jira: "bill-881".into(),
            repo: "billing-svc".into(),
            tags: "backend,perf".into(),
            due: "2030-01-15".into(),
            ..Default::default()
        })
        .unwrap();
    assert_eq!(added["id"], 1);
    assert_eq!(added["jira"], "BILL-881");
    assert_eq!(added["priority"], 1);
    t.add(&NewTask { title: "-starts with a dash".into(), ..Default::default() }).unwrap();

    let list = t.list().unwrap();
    assert_eq!(ids(&list), vec![1, 2]);
    assert_eq!(list[1]["title"], "-starts with a dash");
    // Changes from the app are the user's: no session linked even though the
    // parent environment could have one.
    assert!(t.show(1).unwrap().get("sessions").is_none());

    // An agent reports from a Claude session with a question.
    e.as_agent(
        "sess-42",
        &[
            "report", "1", "--done", "Backoff, 5 tries", "--verified", "212 passed",
            "--needs-you", "Alert #oncall or #billing-alerts?", "--next", "Open PR",
        ],
    );
    let task = t.show(1).unwrap();
    assert_eq!(task["needs_you"], true);
    assert_eq!(task["unread"], true);
    assert_eq!(task["report"]["done"], "Backoff, 5 tries");
    assert_eq!(task["report"]["needs_you"], "Alert #oncall or #billing-alerts?");
    assert_eq!(task["report"]["session_id"], "sess-42");
    assert_eq!(task["sessions"][0]["id"], "sess-42");
    assert_eq!(task["sessions"][0]["name"], "webhook retry");
    assert_eq!(task["sessions"][0]["launch_dir"], "/work");

    t.answer(1, "billing-alerts").unwrap();
    let task = t.show(1).unwrap();
    assert!(task.get("needs_you").is_none());
    assert_eq!(task["report"]["answer"], "billing-alerts");
    // Answering again changes the answer.
    t.answer(1, "billing-oncall").unwrap();
    assert_eq!(t.show(1).unwrap()["report"]["answer"], "billing-oncall");

    t.note(1, "-v is fine as a note").unwrap();
    let log = t.show(1).unwrap()["log"].as_array().unwrap().clone();
    let last = log.last().unwrap();
    assert_eq!(last["text"], "-v is fine as a note");
    assert_eq!(last["author"], "user");

    let edited = t
        .edit(
            1,
            &TaskEdit {
                title: Some("Retry webhooks".into()),
                jira: Some("".into()),
                due: Some("none".into()),
                priority: Some("low".into()),
                tag: vec!["urgent".into()],
                untag: vec!["perf".into()],
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(edited["title"], "Retry webhooks");
    assert!(edited.get("jira").is_none());
    assert!(edited.get("due").is_none());
    assert_eq!(edited["priority"], 3);
    assert_eq!(edited["tags"], serde_json::json!(["backend", "urgent"]));
    assert_eq!(t.edit(1, &TaskEdit::default()).unwrap()["title"], "Retry webhooks");

    let repo = t.set_repo("billing-svc", "~/code/billing", Some(Path::new("/home/me"))).unwrap();
    assert_eq!(repo["path"], "/home/me/code/billing");
    assert_eq!(t.repos().unwrap()[0]["name"], "billing-svc");
    t.remove_repo("billing-svc").unwrap();
    assert_eq!(t.repos().unwrap(), Value::Array(vec![]));

    let moved = t.mv(1, "in-review").unwrap();
    assert_eq!(moved[0]["status"], "in-review");
    let done = t.done(2).unwrap();
    assert_eq!(done[0]["status"], "done");
    let reopened = t.mv(2, "inbox").unwrap();
    assert_eq!(reopened[0]["status"], "inbox");

    let err = t.mv(1, "sideways").unwrap_err();
    assert_eq!(err.kind, ErrorKind::Failed);
    assert!(err.message.starts_with("unknown status \"sideways\""), "{}", err.message);
    t.delete(2).unwrap();
    assert_eq!(ids(&t.list().unwrap()), vec![1]);
    assert_eq!(t.delete(2).unwrap_err().kind, ErrorKind::Failed);
    assert_eq!(t.show(99).unwrap_err().message, "task 99: not found");
}

#[test]
fn db_flag_overrides_tars_home() {
    let e = env();
    let mut t = e.tars.clone();
    let db = e.home.parent().unwrap().join("elsewhere.db");
    t.db = Some(db.clone());
    t.add(&NewTask { title: "In the other db".into(), ..Default::default() }).unwrap();
    assert!(db.exists());
    assert_eq!(ids(&t.list().unwrap()), vec![1]);
    assert_eq!(e.tars.list().unwrap(), Value::Array(vec![]));
}

#[test]
fn detect_skips_binaries_that_are_too_old() {
    let e = env();
    let dir = tempfile::tempdir().unwrap();
    // A stand-in for tars 0.1.3, which has no --needs-you flag.
    let old = dir.path().join("tars-old");
    std::fs::write(&old, "#!/bin/sh\necho '✘ unknown flag: --needs-you' >&2\nexit 1\n").unwrap();
    Command::new("chmod").arg("+x").arg(&old).status().unwrap();

    let envs = e.tars.envs.clone();
    let configure = |t: &mut Tars| t.envs = envs.clone();
    let err = cli::detect(vec![old.clone()], configure).unwrap_err();
    assert_eq!(err.kind, ErrorKind::TooOld);
    assert!(err.message.contains("tars-old is too old"), "{}", err.message);

    let found = cli::detect(vec![old, tars_bin().to_path_buf()], configure).unwrap();
    assert_eq!(found.bin, tars_bin());
    assert_eq!(found.version().unwrap(), "dev");
}
