import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { MotionConfig } from "motion/react";
import { App } from "./App";
import "./styles.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    {/* reducedMotion="user" turns transforms off for visitors who ask for less motion. */}
    <MotionConfig reducedMotion="user" transition={{ duration: 0.5, ease: [0.2, 0.7, 0.2, 1] }}>
      <App />
    </MotionConfig>
  </StrictMode>,
);
