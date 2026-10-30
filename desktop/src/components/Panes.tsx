import { createContext, useContext } from "react";

// Whether the sidebar and preview are showing, for the buttons that toggle
// them from the list and page headers.
export interface Panes {
  sidebarOpen: boolean;
  detailOpen: boolean;
  toggleSidebar: () => void;
  toggleDetail: () => void;
}

export const PanesContext = createContext<Panes>({
  sidebarOpen: true,
  detailOpen: true,
  toggleSidebar: () => {},
  toggleDetail: () => {},
});

export function SidebarToggle() {
  const { sidebarOpen, toggleSidebar } = useContext(PanesContext);
  const label = sidebarOpen ? "Hide sidebar" : "Show sidebar";
  return (
    <button className={"icon-button pane-toggle" + (sidebarOpen ? "" : " off")} onClick={toggleSidebar} aria-label={label} title={`${label} (⌘B)`}>
      <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
        <rect x="1.75" y="2.75" width="12.5" height="10.5" rx="2" fill="none" stroke="currentColor" strokeWidth="1.3" />
        <path d="M6 3v10" stroke="currentColor" strokeWidth="1.3" />
        {sidebarOpen && <rect x="2.5" y="3.5" width="3.5" height="9" rx="1" fill="currentColor" opacity="0.25" />}
      </svg>
    </button>
  );
}

export function PreviewToggle() {
  const { detailOpen, toggleDetail } = useContext(PanesContext);
  const label = detailOpen ? "Hide preview" : "Show preview";
  return (
    <button className={"icon-button pane-toggle" + (detailOpen ? "" : " off")} onClick={toggleDetail} aria-label={label} title={`${label} (⌘⇧B)`}>
      <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
        <rect x="1.75" y="2.75" width="12.5" height="10.5" rx="2" fill="none" stroke="currentColor" strokeWidth="1.3" />
        <path d="M9.5 3v10" stroke="currentColor" strokeWidth="1.3" />
        {detailOpen && <rect x="9.5" y="3.5" width="4" height="9" rx="1" fill="currentColor" opacity="0.25" />}
      </svg>
    </button>
  );
}
