import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { fetchSemesters, sameSemester, semesterLabel } from "./semesters";

describe("semesterLabel", () => {
  /**
   * Must stay identical to Semester.FriendlyString in web/ts/api/semesters.ts while
   * both frontends are up: a winter semester spans the turn of the year and is named
   * with both, and students recognise `Winter 2026/27` rather than `Winter 2026`.
   */
  it.each([
    [{ year: 2026, term: "W" } as const, "Winter 2026/27"],
    [{ year: 2025, term: "S" } as const, "Summer 2025"],
    // The two-digit second year is a string slice, so a century boundary is worth one.
    [{ year: 2099, term: "W" } as const, "Winter 2099/00"],
  ])("renders %o as %s", (semester, expected) => {
    expect(semesterLabel(semester)).toBe(expected);
  });
});

describe("sameSemester", () => {
  it("is false when either side is missing rather than treating them as equal", () => {
    expect(sameSemester(undefined, undefined)).toBe(false);
    expect(sameSemester({ year: 2026, term: "W" }, undefined)).toBe(false);
  });

  it("compares both halves", () => {
    expect(sameSemester({ year: 2026, term: "W" }, { year: 2026, term: "W" })).toBe(true);
    expect(sameSemester({ year: 2026, term: "W" }, { year: 2026, term: "S" })).toBe(false);
    expect(sameSemester({ year: 2026, term: "W" }, { year: 2025, term: "W" })).toBe(false);
  });
});

describe("fetchSemesters", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    clearToken();
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => vi.unstubAllGlobals());

  function respond(): void {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          current: { year: 2026, teachingTerm: "W" },
          semesters: [
            { year: 2026, teachingTerm: "W" },
            { year: 2026, teachingTerm: "S" },
          ],
        }),
        { status: 200 },
      ),
    );
  }

  /**
   * The start page waits on this before it loads anything else, so asking for a token
   * here failed the whole page for a visitor who is not signed in. GetSemesters never
   * looks at the caller, so there is nothing a token would add.
   */
  it("asks for no token, so it answers a visitor who is not signed in", async () => {
    respond();

    await fetchSemesters();

    expect(fetchMock.mock.calls.map((call) => String(call[0]))).toEqual(["/api/v2/semesters"]);
  });

  it("reads the list and the current semester", async () => {
    respond();

    const { current, all } = await fetchSemesters();

    expect(current).toEqual({ year: 2026, term: "W" });
    expect(all).toEqual([
      { year: 2026, term: "W" },
      { year: 2026, term: "S" },
    ]);
  });
});
