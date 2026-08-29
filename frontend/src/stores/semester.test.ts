import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSemesterStore } from "./semester";

/**
 * The store holds the semester list; which semester a page is about stays in the URL.
 * What is worth covering is that the list is fetched once however many components ask,
 * and that `resolve` prefers the URL over the current semester without ever inventing
 * half of one.
 */

const { fetchSemesters } = vi.hoisted(() => ({ fetchSemesters: vi.fn() }));

vi.mock("@/lib/semesters", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/semesters")>()),
  fetchSemesters,
}));

const current = { year: 2026, term: "W" } as const;
const all = [current, { year: 2026, term: "S" } as const];

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  fetchSemesters.mockResolvedValue({ current, all });
});

describe("the semester store", () => {
  it("shares one request between callers that ask at the same time", async () => {
    const store = useSemesterStore();

    await Promise.all([store.load(), store.load()]);

    expect(fetchSemesters).toHaveBeenCalledTimes(1);
    expect(store.current).toEqual(current);
    expect(store.semesters).toEqual(all);
  });

  it("does not ask again once loaded", async () => {
    const store = useSemesterStore();

    await store.load();
    await store.load();

    expect(fetchSemesters).toHaveBeenCalledTimes(1);
  });

  it("stays unloaded after a failure, so the next caller retries", async () => {
    const store = useSemesterStore();
    fetchSemesters.mockRejectedValueOnce(new Error("API is down"));

    await expect(store.load()).rejects.toThrow("API is down");
    expect(store.loaded).toBe(false);

    await store.load();
    expect(store.loaded).toBe(true);
    expect(fetchSemesters).toHaveBeenCalledTimes(2);
  });
});

describe("resolve", () => {
  it("prefers what the URL says", async () => {
    const store = useSemesterStore();
    await store.load();

    expect(store.resolve(2024, "S")).toEqual({ year: 2024, term: "S" });
  });

  it("honours a semester that is not in the list", async () => {
    // The list is the semesters that have courses. Asking for an empty one should
    // show it empty rather than silently redirecting to the current semester.
    const store = useSemesterStore();
    await store.load();

    expect(store.resolve(1999, "W")).toEqual({ year: 1999, term: "W" });
    expect(store.isKnown({ year: 1999, term: "W" })).toBe(false);
    expect(store.isKnown(current)).toBe(true);
  });

  it("falls back to the current semester when the URL names none", async () => {
    const store = useSemesterStore();
    await store.load();

    expect(store.resolve()).toEqual(current);
    // Half a semester is not a semester; the year alone would ask for the wrong term.
    expect(store.resolve(2024, undefined)).toEqual(current);
  });

  it("is undefined before the current semester is known", () => {
    // Which the API reads as "the current semester" anyway, so a page can render
    // before the store has loaded rather than waiting on it.
    expect(useSemesterStore().resolve()).toBeUndefined();
  });
});
