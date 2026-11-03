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
                pr: vec!["https://github.com/acme/billing/pull/412".into()],
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
    assert_eq!(edited["prs"], serde_json::json!(["https://github.com/acme/billing/pull/412"]));
    assert_eq!(edited["tags"], serde_json::json!(["backend", "urgent"]));
    assert_eq!(t.edit(1, &TaskEdit::default()).unwrap()["title"], "Retry webhooks");

    let repo = t.set_repo("billing-svc", "~/code/billing", Some(Path::new("/home/me"))).unwrap();
    assert_eq!(repo["path"], "/home/me/code/billing");
    assert_eq!(t.repos().unwrap()[0]["name"], "billing-svc");
    e.as_agent("sess-42", &["repo", "set", "billing-svc", "-S", "tests", "-x", "make test"]);
    let shown = t.show_repo("billing-svc").unwrap();
    assert_eq!(shown["sections"][0]["name"], "tests");
    assert_eq!(shown["sections"][0]["content"], "make test");
    assert_eq!(t.profile().unwrap(), Value::Array(vec![]));
    e.as_agent("sess-42", &["profile", "set", "style", "-x", "small diffs"]);
    assert_eq!(t.profile().unwrap()[0]["content"], "small diffs");
    t.set_repo_section("billing-svc", "gotchas", "-v needs docker\n- second").unwrap();
    t.set_profile_section("style", "").unwrap();
    assert_eq!(t.profile().unwrap(), Value::Array(vec![]));
    t.add(&NewTask { title: "in billing".into(), repo: "billing-svc".into(), ..Default::default() }).unwrap();
    let renamed = t.rename_repo("billing-svc", "billing").unwrap();
    assert_eq!(renamed["name"], "billing");
    let shown = t.show_repo("billing").unwrap();
    let gotchas = shown["sections"].as_array().unwrap().iter().find(|s| s["name"] == "gotchas").unwrap().clone();
    assert_eq!(gotchas["content"], "-v needs docker\n- second");
    let moved = t.list().unwrap().as_array().unwrap().iter().find(|x| x["title"] == "in billing").unwrap().clone();
    assert_eq!(moved["repo"], "billing");
    t.delete(moved["id"].as_i64().unwrap()).unwrap();
    t.set_repo_section("billing", "gotchas", "").unwrap();
    assert_eq!(t.show_repo("billing").unwrap()["sections"].as_array().unwrap().len(), 1);
    t.remove_repo("billing").unwrap();
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

impl Env {
    /// Feeds a Claude Code hook event to tars, as the plugin's hooks do.
    fn hook(&self, event: &str, input: &Value) {
        use std::io::Write;
        let mut child = Command::new(tars_bin())
            .args(["hook", event])
            .env("TARS_HOME", &self.home)
            .env("CLAUDE_CONFIG_DIR", &self.claude)
            .stdin(std::process::Stdio::piped())
            .stdout(std::process::Stdio::null())
            .spawn()
            .unwrap();
        child.stdin.take().unwrap().write_all(input.to_string().as_bytes()).unwrap();
        assert!(child.wait().unwrap().success());
    }
}

#[test]
fn files_captured_sessions_and_merges_tasks() {
    let e = env();
    let t = &e.tars;
    let repo = e.home.parent().unwrap().join("billing");
    for args in [
        vec!["init", "-q", "-b", "main", repo.to_str().unwrap()],
        vec!["-C", repo.to_str().unwrap(), "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "x"],
    ] {
        assert!(Command::new("git").args(&args).status().unwrap().success());
    }
    let transcript = e.home.parent().unwrap().join("t.jsonl");
    std::fs::write(
        &transcript,
        "{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"tool_use\",\"name\":\"Write\",\"input\":{\"file_path\":\"/x\"}}]}}\n",
    )
    .unwrap();
    for s in ["sess-a", "sess-b"] {
        let input = serde_json::json!({"session_id": s, "cwd": repo, "prompt": format!("Speed up exports {s}"), "transcript_path": transcript});
        e.hook("prompt", &input);
        e.hook("end", &input);
    }

    let unfiled = t.unfiled().unwrap();
    assert_eq!(unfiled.as_array().unwrap().len(), 2);
    assert_eq!(unfiled[0]["repo"], "billing");
    assert_eq!(unfiled[0]["edits"], 1);
    assert_eq!(t.captured().unwrap().as_array().unwrap().len(), 2);

    let filed = t.file_session("sess-a", None, "Faster exports").unwrap();
    let id = filed["task_id"].as_i64().unwrap();
    assert_eq!(t.show(id).unwrap()["title"], "Faster exports");
    t.dismiss("sess-b").unwrap();
    assert_eq!(t.unfiled().unwrap(), Value::Array(vec![]));

    let other = t.add(&NewTask { title: "Exports, again".into(), ..Default::default() }).unwrap()["id"].as_i64().unwrap();
    t.file_session("sess-b", Some(other), "").unwrap();
    t.merge(other, id).unwrap();
    let merged = t.show(id).unwrap();
    assert_eq!(merged["sessions"].as_array().unwrap().len(), 2);
    assert_eq!(merged["sessions"][0]["via"], "user");
    t.unlink(id, "sess-b").unwrap();
    assert_eq!(t.show(id).unwrap()["sessions"].as_array().unwrap().len(), 1);
    assert_eq!(t.unfiled().unwrap()[0]["session_id"], "sess-b");
    let captured = t.captured().unwrap();
    assert_eq!(captured.as_array().unwrap().len(), 2);
    assert!(captured.as_array().unwrap().iter().any(|c| c["state"] == "linked" && c["task_id"] == id));
    assert_eq!(t.show(other).unwrap_err().kind, ErrorKind::Failed);

    assert_eq!(t.capture(None, None).unwrap()["on"], true);
    assert_eq!(t.capture(Some(false), None).unwrap()["on"], false);
    t.set_repo("billing", repo.to_str().unwrap(), None).unwrap();
    assert_eq!(t.capture(Some(false), Some("billing")).unwrap()["off_repos"], serde_json::json!(["billing"]));
    assert_eq!(t.repos().unwrap()[0]["capture_off"], true);
}

#[test]
fn todo_lists_round_trip_through_the_cli() {
    let e = env();
    let t = &e.tars;
    assert_eq!(t.todos().unwrap(), Value::Array(vec![]));

    let g = t.todo_new_list(" Release ").unwrap();
    assert_eq!(g["name"], "Release");
    let gid = g["id"].as_i64().unwrap();
    let h = t.todo_new_list("Cleanup").unwrap()["id"].as_i64().unwrap();
    // The CLI's own checks reach the app as plain messages.
    let dup = t.todo_new_list("release").unwrap_err();
    assert_eq!(dup.kind, ErrorKind::Failed);
    assert_eq!(dup.message, "there's already a todo list called \"release\"");
    assert!(t.todo_new_list("-x").unwrap_err().message.contains("can't start with"));

    let race = t.todo_add(gid, " -race in CI ").unwrap();
    assert_eq!(race["text"], "-race in CI");
    let race = race["id"].as_i64().unwrap();
    let tag = t.todo_add(gid, "Tag v0.4.0").unwrap()["id"].as_i64().unwrap();
    assert_eq!(t.todo_edit(tag, "-").unwrap()["text"], "-");
    assert_eq!(t.todo_check(race, true).unwrap()[0]["done"], true);
    t.todo_move(tag, 1, None).unwrap();
    t.todo_move(race, 1, Some(h)).unwrap();
    t.todo_move_list(h, 1).unwrap();
    t.todo_rename_list(gid, "Ship it").unwrap();

    let ls = t.todos().unwrap();
    assert_eq!(ls[0]["name"], "Cleanup");
    assert_eq!(ls[0]["items"][0]["text"], "-race in CI");
    assert_eq!(ls[1]["name"], "Ship it");
    assert_eq!(ls[1]["items"][0]["text"], "-");
    assert_eq!(t.todo_move(tag, 9, None).unwrap_err().kind, ErrorKind::Failed);

    assert_eq!(t.todo_clear(h).unwrap()["cleared"], 1);
    let err = t.todo_remove_list(gid, false).unwrap_err();
    assert!(err.message.contains("--force"), "{}", err.message);
    t.todo_remove_list(gid, true).unwrap();
    t.todo_remove_list(h, false).unwrap();
    assert_eq!(t.todos().unwrap(), Value::Array(vec![]));
    assert_eq!(t.todo_remove(tag).unwrap_err().message, format!("todo {tag}: not found"));
}
