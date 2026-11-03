fn main() {
    // The Tars Dev build (TARS_FLAVOR=dev) drives tars-dev; rebuild when it changes.
    println!("cargo:rerun-if-env-changed=TARS_FLAVOR");
    tauri_build::build()
}
