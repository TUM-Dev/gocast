import type { PlayerLike } from "./types";

export interface ContextMenuItem {
  label: () => string;
  run(): void;
}

/**
 * A right-click menu over the picture, for actions that have no control-bar button.
 * video.js has none of its own. Shift + right-click is left to the browser, so "save
 * video as" and "inspect" stay reachable. Returns the function that removes it.
 */
export function attachContextMenu(player: Pick<PlayerLike, "el">, items: ContextMenuItem[]): () => void {
  const host = player.el();
  const menu = document.createElement("ul");
  menu.className = "vjs-context-menu";
  menu.setAttribute("role", "menu");
  menu.hidden = true;
  host.appendChild(menu);

  const hide = () => {
    menu.hidden = true;
    document.removeEventListener("pointerdown", onOutside, true);
    document.removeEventListener("keydown", onKey, true);
  };
  const onOutside = (e: Event) => {
    if (!menu.contains(e.target as Node)) hide();
  };
  const onKey = (e: Event) => {
    if ((e as KeyboardEvent).key === "Escape") hide();
  };

  const onContextMenu = (e: Event) => {
    const event = e as MouseEvent;
    if (event.shiftKey) return;
    event.preventDefault();
    menu.replaceChildren(
      ...items.map((item) => {
        const li = document.createElement("li");
        li.setAttribute("role", "none");
        const button = document.createElement("button");
        button.type = "button";
        button.setAttribute("role", "menuitem");
        button.textContent = item.label();
        button.addEventListener("click", () => {
          hide();
          item.run();
        });
        li.appendChild(button);
        return li;
      }),
    );
    const box = host.getBoundingClientRect();
    menu.style.left = `${Math.max(0, event.clientX - box.left)}px`;
    menu.style.top = `${Math.max(0, event.clientY - box.top)}px`;
    menu.hidden = false;
    document.addEventListener("pointerdown", onOutside, true);
    document.addEventListener("keydown", onKey, true);
    menu.querySelector<HTMLButtonElement>("button")?.focus();
  };

  host.addEventListener("contextmenu", onContextMenu);
  return () => {
    hide();
    host.removeEventListener("contextmenu", onContextMenu);
    menu.remove();
  };
}
