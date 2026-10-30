pub mod cli;
pub mod mac;

use std::path::PathBuf;
use std::sync::Mutex;

use serde::Serialize;
use serde_json::Value;
use tauri::State;

use cli::{CliError, ErrorKind, NewTask, Result, Tars};

/// The tars binary in use, found on first need and again after a failure to
/// run it, so installing or upgrading the CLI doesn't need an app restart.
#[derive(Default)]
struct AppState {
    tars: Mutex<Option<Tars>>,
}

fn home() -> Option<PathBuf> {
    std::env::var_os("HOME").map(PathBuf::from)
}

fn find_tars() -> Result<Tars> {
    let tars_bin = std::env::var("TARS_BIN").ok();
    let path = std::env::var("PATH").ok();
    let home = home();
    let found = cli::candidates(tars_bin.as_deref(), path.as_deref(), home.as_deref(), |p| p.is_file());
    let db = std::env::var_os("TARS_DB").filter(|d| !d.is_empty()).map(PathBuf::from);
    cli::detect(found, |t| t.db = db.clone())
}

fn current(state: &AppState) -> Result<Tars> {
    let mut slot = state.tars.lock().unwrap();
    if let Some(t) = slot.as_ref() {
        return Ok(t.clone());
    }
    let t = find_tars()?;
    *slot = Some(t.clone());
    Ok(t)
}

/// Runs f against the current binary off the main thread. Missing or outdated
/// binaries are forgotten so the next call looks again.
async fn with_tars<T: Send + 'static>(
    state: State<'_, AppState>,
    f: impl FnOnce(&Tars) -> Result<T> + Send + 'static,
) -> Result<T> {
    let t = current(&state)?;
    let res = tauri::async_runtime::spawn_blocking(move || f(&t))
        .await
        .map_err(|e| CliError::new(ErrorKind::System, e.to_string()))?;
    if let Err(e) = &res {
        if matches!(e.kind, ErrorKind::Missing | ErrorKind::TooOld) {
            *state.tars.lock().unwrap() = None;
        }
    }
    res
}

async fn blocking<T: Send + 'static>(f: impl FnOnce() -> Result<T> + Send + 'static) -> Result<T> {
    tauri::async_runtime::spawn_blocking(f)
        .await
        .map_err(|e| CliError::new(ErrorKind::System, e.to_string()))?
}

#[derive(Serialize)]
struct CliStatus {
    bin: String,
    version: String,
}

#[tauri::command]
async fn cli_status(state: State<'_, AppState>) -> Result<CliStatus> {
    *state.tars.lock().unwrap() = None;
    with_tars(state, |t| Ok(CliStatus { bin: t.bin.display().to_string(), version: t.version()? })).await
}

#[tauri::command]
async fn list_tasks(state: State<'_, AppState>) -> Result<Value> {
    with_tars(state, |t| t.list()).await
}

#[tauri::command]
async fn show_task(state: State<'_, AppState>, id: i64) -> Result<Value> {
    with_tars(state, move |t| t.show(id)).await
}

#[tauri::command]
async fn add_task(state: State<'_, AppState>, task: NewTask) -> Result<Value> {
    with_tars(state, move |t| t.add(&task)).await
}

#[tauri::command]
async fn set_status(state: State<'_, AppState>, id: i64, status: String) -> Result<Value> {
    with_tars(state, move |t| if status == "done" { t.done(id) } else { t.mv(id, &status) }).await
}

#[tauri::command]
async fn answer_task(state: State<'_, AppState>, id: i64, text: String) -> Result<Value> {
    with_tars(state, move |t| t.answer(id, &text)).await
}

#[tauri::command]
async fn add_note(state: State<'_, AppState>, id: i64, text: String) -> Result<Value> {
    with_tars(state, move |t| t.note(id, &text)).await
}

#[tauri::command]
async fn mark_seen(state: State<'_, AppState>, id: i64) -> Result<Value> {
    with_tars(state, move |t| t.seen(id)).await
}

#[tauri::command]
async fn copy_text(text: String) -> Result<()> {
    blocking(move || mac::copy_text(&text)).await
}

#[tauri::command]
async fn open_in_terminal(session_id: String, launch_dir: Option<String>) -> Result<()> {
    blocking(move || mac::open_in_terminal(&session_id, launch_dir.as_deref())).await
}

#[tauri::command]
async fn reveal_path(path: String) -> Result<()> {
    blocking(move || mac::reveal(&path, home().as_deref())).await
}

#[tauri::command]
async fn open_url(url: String) -> Result<()> {
    blocking(move || mac::open_url(&url)).await
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .manage(AppState::default())
        .invoke_handler(tauri::generate_handler![
            cli_status,
            list_tasks,
            show_task,
            add_task,
            set_status,
            answer_task,
            add_note,
            mark_seen,
            copy_text,
            open_in_terminal,
            reveal_path,
            open_url,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tars");
}
