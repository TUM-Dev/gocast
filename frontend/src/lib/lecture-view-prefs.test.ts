import { afterEach, describe, expect, it } from "vitest";
import { nextTick } from "vue";

import { useLectureViewPrefs } from "./lecture-view-prefs";

/**
 * The choice is remembered per browser under `spa.` keys, in words rather than the
 * legacy page's numeric enums. What matters is that an unknown word is discarded
 * instead of rendered, and that every change lands under its own key.
 */

afterEach(() => localStorage.clear());

describe("useLectureViewPrefs", () => {
  it("starts from the defaults when nothing is stored", () => {
    const prefs = useLectureViewPrefs();

    expect(prefs.sort.value).toBe("newest");
    expect(prefs.group.value).toBe("month");
    expect(prefs.view.value).toBe("grid");
    expect(prefs.hideWatched.value).toBe(false);
    expect(localStorage.getItem("spa.lectureHideWatched")).toBeNull();
  });

  it("restores every choice from its own key", () => {
    localStorage.setItem("spa.lectureSort", "oldest");
    localStorage.setItem("spa.lectureGroup", "week");
    localStorage.setItem("spa.lectureView", "list");
    localStorage.setItem("spa.lectureHideWatched", "yes");

    const prefs = useLectureViewPrefs();

    expect(prefs.sort.value).toBe("oldest");
    expect(prefs.group.value).toBe("week");
    expect(prefs.view.value).toBe("list");
    expect(prefs.hideWatched.value).toBe(true);
  });

  it("discards a stored value that is not one of its words", () => {
    // The legacy page wrote numeric enums, and "true" is not how the boolean is stored.
    localStorage.setItem("spa.lectureSort", "1");
    localStorage.setItem("spa.lectureGroup", "semester");
    localStorage.setItem("spa.lectureView", "table");
    localStorage.setItem("spa.lectureHideWatched", "true");

    const prefs = useLectureViewPrefs();

    expect(prefs.sort.value).toBe("newest");
    expect(prefs.group.value).toBe("month");
    expect(prefs.view.value).toBe("grid");
    expect(prefs.hideWatched.value).toBe(false);
  });

  it("writes each change back under its spa key", async () => {
    const prefs = useLectureViewPrefs();

    prefs.sort.value = "oldest";
    prefs.group.value = "week";
    prefs.view.value = "list";
    prefs.hideWatched.value = true;
    await nextTick();

    expect(localStorage.getItem("spa.lectureSort")).toBe("oldest");
    expect(localStorage.getItem("spa.lectureGroup")).toBe("week");
    expect(localStorage.getItem("spa.lectureView")).toBe("list");
    expect(localStorage.getItem("spa.lectureHideWatched")).toBe("yes");

    prefs.hideWatched.value = false;
    await nextTick();

    expect(localStorage.getItem("spa.lectureHideWatched")).toBe("no");
  });
});
