import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { isTauri } from "@tauri-apps/api/core";
import "@fontsource-variable/geist";
import "@fontsource-variable/geist-mono";
import "@fontsource/instrument-serif";
import "./styles.css";
import { App } from "./App";
import { OutsideApp } from "./components/Dialogs";

// The UI only works inside the tars window, which provides the bridge to the
// CLI. Opened in a browser (e.g. the dev server's URL), say so plainly.
createRoot(document.getElementById("root")!).render(
  <StrictMode>{isTauri() ? <App /> : <OutsideApp />}</StrictMode>,
);
