/** The sidebar's tabs, and which one the viewer last chose. */

export const SIDEBAR_TABS = [
  { id: "chat", label: "Chat" },
  { id: "bookmarks", label: "Bookmarks" },
  { id: "sections", label: "Sections" },
  { id: "playlist", label: "Playlist" },
  { id: "transcript", label: "Transcript" },
  { id: "search", label: "Search" },
] as const;

export type SidebarTab = (typeof SIDEBAR_TABS)[number]["id"];

const KEY = "watch.sidebarTab";

/** Storage can be missing or throw (private windows, blocked site data); never let it matter. */
export function readSidebarTab(): SidebarTab | null {
  try {
    const value = localStorage.getItem(KEY);
    return SIDEBAR_TABS.some((t) => t.id === value) ? (value as SidebarTab) : null;
  } catch {
    return null;
  }
}

export function writeSidebarTab(tab: SidebarTab): void {
  try {
    localStorage.setItem(KEY, tab);
  } catch {
    // The choice just is not remembered.
  }
}
