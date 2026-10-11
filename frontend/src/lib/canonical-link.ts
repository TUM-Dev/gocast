import { onBeforeUnmount, toValue, watch, type MaybeRefOrGetter } from "vue";

/**
 * Points `<link rel="canonical">` at `href` and returns the function that undoes it:
 * the tag is removed again if this created it, or put back to what it said before.
 */
export function setCanonicalLink(href: string): () => void {
  const existing = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]');
  const link = existing ?? document.createElement("link");
  const previous = existing?.getAttribute("href") ?? null;
  link.rel = "canonical";
  link.href = href;
  if (!existing) document.head.appendChild(link);
  return () => {
    if (!existing) link.remove();
    else if (previous === null) link.removeAttribute("href");
    else link.setAttribute("href", previous);
  };
}

/**
 * Keeps the page's canonical link in step with `href` for as long as the calling
 * component lives. An undefined `href` means "none yet" (the page is still loading, or
 * the base URL is not known): no tag is written for it. The SPA's shell is one document
 * for every route, so a page that sets a canonical must also take it away on leaving, or
 * the next page would claim to be the previous one.
 */
export function useCanonicalLink(href: MaybeRefOrGetter<string | undefined>): void {
  let undo: (() => void) | null = null;
  watch(
    () => toValue(href),
    (value) => {
      undo?.();
      undo = value ? setCanonicalLink(value) : null;
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    undo?.();
    undo = null;
  });
}

/**
 * `base` + path segments, each escaped as one segment, without a trailing slash. With no
 * configured base the page's own origin is used: a canonical link must be absolute, and
 * a deployment that never set one is still served from one place.
 */
export function canonicalUrl(base: string, ...segments: (string | number | undefined)[]): string {
  const root = (base || window.location.origin).replace(/\/+$/, "");
  const path = segments
    .filter((s): s is string | number => s !== undefined && s !== "")
    .map((s) => encodeURIComponent(String(s)));
  return [root, ...path].join("/");
}
