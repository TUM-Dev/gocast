import { describe, expect, it, vi } from "vitest";

// The composable reads the current route; stubbing it here keeps the test to the
// parsing, which is the part with rules of its own. Same approach as LoginView.test.
vi.mock("vue-router", () => ({ useRoute: () => mockRoute }));
let mockRoute: { query: Record<string, unknown> } = { query: {} };

const { useSemesterQuery, singleQueryParam } = await import("./route-query");

function semesterFor(query: Record<string, unknown>) {
  mockRoute = { query };
  const { year, term } = useSemesterQuery();
  return { year: year.value, term: term.value };
}

describe("singleQueryParam", () => {
  it("reads a parameter given once", () => {
    expect(singleQueryParam({ year: "2026" }, "year")).toBe("2026");
  });

  it.each([
    ["absent", {}],
    ["empty", { year: "" }],
    // `?year=2025&year=2026` — no reason to prefer either.
    ["repeated", { year: ["2025", "2026"] }],
    // `?year` with no value at all.
    ["valueless", { year: null }],
  ])("reports a %s parameter as undefined", (_name, query) => {
    expect(singleQueryParam(query, "year")).toBeUndefined();
  });
});

describe("useSemesterQuery", () => {
  it("reads a well-formed semester", () => {
    expect(semesterFor({ year: "2026", term: "W" })).toEqual({ year: 2026, term: "W" });
    expect(semesterFor({ year: "2025", term: "S" })).toEqual({ year: 2025, term: "S" });
  });

  it("reports an absent semester as undefined, meaning the current one", () => {
    expect(semesterFor({})).toEqual({ year: undefined, term: undefined });
  });

  it.each([
    ["a term that is not W or S", { year: "2026", term: "X" }, { year: 2026, term: undefined }],
    ["a lowercase term", { year: "2026", term: "w" }, { year: 2026, term: undefined }],
    ["a year that is not a number", { year: "soon", term: "W" }, { year: undefined, term: "W" }],
    // Number("2026.5") parses; Number.isInteger is what rejects it.
    ["a fractional year", { year: "2026.5", term: "W" }, { year: undefined, term: "W" }],
    ["a negative year", { year: "-2026", term: "W" }, { year: undefined, term: "W" }],
    // Number("") is 0, which would otherwise read as a real year.
    ["a blank year", { year: "", term: "W" }, { year: undefined, term: "W" }],
  ])("discards %s", (_name, query, expected) => {
    expect(semesterFor(query)).toEqual(expected);
  });
});
