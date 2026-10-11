/**
 * The brief icon that flashes over the picture when a hotkey acts (play, volume, skip),
 * ported from the old player's OverlayIcon component. It is plain DOM rather than a
 * video.js Component so it needs no registration; the CSS is player.css.
 */
export function createOverlayIcon(host: Element, removeAfterMs = 3000) {
  const container = document.createElement("div");
  container.className = "vjs-overlay-icon-container";
  container.setAttribute("aria-hidden", "true");
  host.appendChild(container);
  let timer: ReturnType<typeof setTimeout> | null = null;

  const clear = () => {
    if (timer !== null) clearTimeout(timer);
    timer = null;
    container.replaceChildren();
    container.classList.remove("vjs-overlay-icon-animate");
  };

  return {
    /** `className` is the icon's class string, e.g. "vjs-icon-play" or "fa-solid fa-forward". */
    show(className: string) {
      clear();
      const wrapper = document.createElement("div");
      wrapper.className = "vjs-overlay-icon-wrapper";
      const icon = document.createElement("i");
      icon.className = className;
      wrapper.appendChild(icon);
      container.appendChild(wrapper);
      container.classList.add("vjs-overlay-icon-animate");
      timer = setTimeout(clear, removeAfterMs);
    },
    dispose() {
      clear();
      container.remove();
    },
  };
}
