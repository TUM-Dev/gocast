import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useCourseStore } from "./courses";

/**
 * The store is what keeps the sidebar and the view beside it showing the same
 * listings from one set of requests. What is worth covering is when it decides to
 * fetch again, that an anonymous visitor is not sent to endpoints that would only
 * 401, and that a pin is not shown until the server has accepted it.
 */

const {
  fetchPublicCourses,
  fetchUserCourses,
  fetchPinnedCourses,
  fetchLiveStreams,
  setCoursePinned,
  hasSession,
} = vi.hoisted(() => ({
  fetchPublicCourses: vi.fn(),
  fetchUserCourses: vi.fn(),
  fetchPinnedCourses: vi.fn(),
  fetchLiveStreams: vi.fn(),
  setCoursePinned: vi.fn(),
  hasSession: vi.fn(),
}));

vi.mock("@/lib/courses", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/courses")>()),
  fetchPublicCourses,
  fetchUserCourses,
  fetchPinnedCourses,
  fetchLiveStreams,
  setCoursePinned,
}));

vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  hasSession,
}));

const course = (id: number, name: string, pinned = false) =>
  ({ id, name, pinned, slug: name.toLowerCase() }) as never;

const winter = { year: 2026, term: "W" } as const;
const summer = { year: 2026, term: "S" } as const;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  hasSession.mockResolvedValue(true);
  fetchPublicCourses.mockResolvedValue([course(1, "Public")]);
  fetchUserCourses.mockResolvedValue([course(2, "Mine")]);
  fetchPinnedCourses.mockResolvedValue([course(3, "Pinned", true)]);
  fetchLiveStreams.mockResolvedValue([]);
  setCoursePinned.mockResolvedValue(undefined);
});

describe("loading", () => {
  it("shares one set of requests between callers that ask at the same time", async () => {
    const store = useCourseStore();

    await Promise.all([store.load(winter), store.load(winter)]);

    expect(fetchPublicCourses).toHaveBeenCalledTimes(1);
    expect(store.publicCourses).toHaveLength(1);
    expect(store.userCourses).toHaveLength(1);
    expect(store.pinnedCourses).toHaveLength(1);
  });

  it("does not fetch again for the semester it is already holding", async () => {
    const store = useCourseStore();

    await store.load(winter);
    await store.load({ ...winter });

    expect(fetchPublicCourses).toHaveBeenCalledTimes(1);
  });

  it("fetches again when the semester changes", async () => {
    const store = useCourseStore();

    await store.load(winter);
    await store.load(summer);

    expect(fetchPublicCourses).toHaveBeenNthCalledWith(2, summer);
    expect(store.semester).toEqual(summer);
  });

  it("stays unloaded after a failure, so the next caller retries", async () => {
    const store = useCourseStore();
    fetchPublicCourses.mockRejectedValueOnce(new Error("API is down"));

    await expect(store.load(winter)).rejects.toThrow("API is down");
    expect(store.loaded).toBe(false);

    await store.load(winter);
    expect(store.loaded).toBe(true);
  });

  it("skips the listings that need a user when nobody is signed in", async () => {
    // Both 401 for an anonymous caller, so asking is two failed requests per page.
    hasSession.mockResolvedValue(false);
    const store = useCourseStore();

    await store.load(winter);

    expect(fetchPublicCourses).toHaveBeenCalledTimes(1);
    expect(fetchUserCourses).not.toHaveBeenCalled();
    expect(fetchPinnedCourses).not.toHaveBeenCalled();
    expect(store.userCourses).toEqual([]);
    expect(store.pinnedCourses).toEqual([]);
  });
});

describe("togglePin", () => {
  it("adds the course to the pinned listing and marks it everywhere", async () => {
    const store = useCourseStore();
    await store.load(winter);

    await store.togglePin(store.publicCourses[0]);

    expect(setCoursePinned).toHaveBeenCalledWith(1, true);
    expect(store.publicCourses[0].pinned).toBe(true);
    expect(store.pinnedCourses.map((c) => c.name)).toEqual(["Pinned", "Public"]);
  });

  it("removes an unpinned course from the pinned listing", async () => {
    const store = useCourseStore();
    await store.load(winter);

    await store.togglePin(store.pinnedCourses[0]);

    expect(setCoursePinned).toHaveBeenCalledWith(3, false);
    expect(store.pinnedCourses).toEqual([]);
  });

  it("leaves the listings alone when the server refuses", async () => {
    // Showing the pin first would leave it in the sidebar looking saved until the
    // next reload.
    const store = useCourseStore();
    await store.load(winter);
    setCoursePinned.mockRejectedValueOnce(new Error("nope"));

    await expect(store.togglePin(store.publicCourses[0])).rejects.toThrow("nope");

    expect(store.publicCourses[0].pinned).toBe(false);
    expect(store.pinnedCourses).toHaveLength(1);
  });
});
