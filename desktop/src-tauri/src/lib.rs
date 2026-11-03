pub mod cli;
pub mod mac;

use std::path::PathBuf;
use std::sync::Mutex;

use serde::Serialize;
use serde_json::Value;
use tauri::State;

use cli::{CliError, ErrorKind, NewTask, Result, Tars, TaskEdit};

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
    let found = cli::candidates(cli::BIN_NAME, tars_bin.as_deref(), path.as_deref(), home.as_deref(), |p| p.is_file());
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
async fn edit_task(state: State<'_, AppState>, id: i64, edit: TaskEdit) -> Result<Value> {
    with_tars(state, move |t| t.edit(id, &edit)).await
}

#[tauri::command]
async fn delete_task(state: State<'_, AppState>, id: i64) -> Result<Value> {
    with_tars(state, move |t| t.delete(id)).await
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
async fn open_in_terminal(session_id: String, launch_dir: Option<String>) -> Result<String> {
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

#[tauri::command]
async fn list_repos(state: State<'_, AppState>) -> Result<Value> {
    with_tars(state, |t| t.repos()).await
}

#[tauri::command]
async fn set_repo(state: State<'_, AppState>, name: String, path: String) -> Result<Value> {
    with_tars(state, move |t| t.set_repo(&name, &path, home().as_deref())).await
}

#[tauri::command]
async fn show_repo(state: State<'_, AppState>, name: String) -> Result<Value> {
    with_tars(state, move |t| t.show_repo(&name)).await
}

#[tauri::command]
async fn get_profile(state: State<'_, AppState>) -> Result<Value> {
    with_tars(state, |t| t.profile()).await
}

#[tauri::command]
async fn rename_repo(state: State<'_, AppState>, from: String, to: String) -> Result<Value> {
    with_tars(state, move |t| t.rename_repo(&from, &to)).await
}

#[tauri::command]
async fn set_repo_section(state: State<'_, AppState>, name: String, section: String, text: String) -> Result<Value> {
    with_tars(state, move |t| t.set_repo_section(&name, &section, &text)).await
}

#[tauri::command]
async fn set_profile_section(state: State<'_, AppState>, section: String, text: String) -> Result<Value> {
    with_tars(state, move |t| t.set_profile_section(&section, &text)).await
}

#[tauri::command]
async fn remove_repo(state: State<'_, AppState>, name: String) -> Result<Value> {
    with_tars(state, move |t| t.remove_repo(&name)).await
}

#[tauri::command]
async fn list_unfiled(state: State<'_, AppState>) -> Result<Value> {
    with_tars(state, |t| t.unfiled()).await
}

#[tauri::command]
async fn list_captured(state: State<'_, AppState>) -> Result<Value> {
    with_tars(state, |t| t.captured()).await
}

#[tauri::command]
async fn file_session(state: State<'_, AppState>, session: String, into: Option<i64>, title: String) -> Result<Value> {
    with_tars(state, move |t| t.file_session(&session, into, &title)).await
}

#[tauri::command]
async fn unlink_session(state: State<'_, AppState>, task: i64, session: String) -> Result<Value> {
    with_tars(state, move |t| t.unlink(task, &session)).await
}

#[tauri::command]
async fn dismiss_session(state: State<'_, AppState>, session: String) -> Result<Value> {
    with_tars(state, move |t| t.dismiss(&session)).await
}

#[tauri::command]
async fn merge_tasks(state: State<'_, AppState>, from: i64, into: i64) -> Result<Value> {
    with_tars(state, move |t| t.merge(from, into)).await
}

#[tauri::command]
async fn capture_settings(state: State<'_, AppState>, on: Option<bool>, repo: Option<String>) -> Result<Value> {
    with_tars(state, move |t| t.capture(on, repo.as_deref())).await
}

#[tauri::command]
async fn usage_summary(state: State<'_, AppState>, days: u32, task: Option<i64>) -> Result<Value> {
    with_tars(state, move |t| t.usage(days, task)).await
}

#[tauri::command]
async fn list_todos(state: State<'_, AppState>) -> Result<Value> {
    with_tars(state, |t| t.todos()).await
}

#[tauri::command]
async fn todo_new_list(state: State<'_, AppState>, name: String) -> Result<Value> {
    with_tars(state, move |t| t.todo_new_list(&name)).await
}

#[tauri::command]
async fn todo_rename_list(state: State<'_, AppState>, id: i64, name: String) -> Result<Value> {
    with_tars(state, move |t| t.todo_rename_list(id, &name)).await
}

#[tauri::command]
async fn todo_move_list(state: State<'_, AppState>, id: i64, position: i64) -> Result<Value> {
    with_tars(state, move |t| t.todo_move_list(id, position)).await
}

#[tauri::command]
async fn todo_remove_list(state: State<'_, AppState>, id: i64, force: bool) -> Result<Value> {
    with_tars(state, move |t| t.todo_remove_list(id, force)).await
}

#[tauri::command]
async fn todo_add(state: State<'_, AppState>, list: i64, text: String) -> Result<Value> {
    with_tars(state, move |t| t.todo_add(list, &text)).await
}

#[tauri::command]
async fn todo_edit(state: State<'_, AppState>, id: i64, text: String) -> Result<Value> {
    with_tars(state, move |t| t.todo_edit(id, &text)).await
}

#[tauri::command]
async fn todo_check(state: State<'_, AppState>, id: i64, done: bool) -> Result<Value> {
    with_tars(state, move |t| t.todo_check(id, done)).await
}

#[tauri::command]
async fn todo_move(state: State<'_, AppState>, id: i64, position: i64, list: Option<i64>) -> Result<Value> {
    with_tars(state, move |t| t.todo_move(id, position, list)).await
}

#[tauri::command]
async fn todo_remove(state: State<'_, AppState>, id: i64) -> Result<Value> {
    with_tars(state, move |t| t.todo_remove(id)).await
}

#[tauri::command]
async fn todo_clear(state: State<'_, AppState>, list: i64) -> Result<Value> {
    with_tars(state, move |t| t.todo_clear(list)).await
}

#[tauri::command]
async fn choose_folder() -> Result<Option<String>> {
    blocking(mac::choose_folder).await
}

#[tauri::command]
fn set_zoom(window: tauri::WebviewWindow, scale: f64) -> Result<()> {
    window
        .set_zoom(scale.clamp(0.5, 2.0))
        .map_err(|e| CliError::new(ErrorKind::System, format!("Couldn't zoom: {e}")))
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
            edit_task,
            delete_task,
            set_status,
            answer_task,
            add_note,
            mark_seen,
            copy_text,
            open_in_terminal,
            reveal_path,
            open_url,
            list_repos,
            set_repo,
            remove_repo,
            show_repo,
            rename_repo,
            set_repo_section,
            set_profile_section,
            get_profile,
            choose_folder,
            set_zoom,
            list_unfiled,
            list_captured,
            file_session,
            dismiss_session,
            unlink_session,
            merge_tasks,
            capture_settings,
            usage_summary,
            list_todos,
            todo_new_list,
            todo_rename_list,
            todo_move_list,
            todo_remove_list,
            todo_add,
            todo_edit,
            todo_check,
            todo_move,
            todo_remove,
            todo_clear,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tars");
}
