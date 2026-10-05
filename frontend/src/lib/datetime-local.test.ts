import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { fromDateTimeLocal, toDateTimeLocal } from "./datetime-local";

/*
 * In a zone with daylight saving, so the DST cases mean something. Node re-reads TZ
 * when it is assigned, so this takes effect for the Dates made below.
 */
let previousTz: string | undefined;

beforeAll(() => {
  previousTz = process.env.TZ;
  process.env.TZ = "Europe/Berlin";
});

afterAll(() => {
  if (previousTz === undefined) delete process.env.TZ;
  else process.env.TZ = previousTz;
});

describe("toDateTimeLocal", () => {
  it("shows the local wall-clock time, not UTC", () => {
    // 12:00 UTC is 14:00 in Berlin in summer.
    expect(toDateTimeLocal(new Date("2026-07-01T12:00:00Z"))).toBe("2026-07-01T14:00");
    // And 13:00 in winter.
    expect(toDateTimeLocal(new Date("2026-01-15T12:00:00Z"))).toBe("2026-01-15T13:00");
  });

  it("pads and drops seconds", () => {
    expect(toDateTimeLocal(new Date(2026, 2, 5, 7, 4, 59))).toBe("2026-03-05T07:04");
  });

  it("keeps an after-midnight lecture on its local day", () => {
    // 00:30 in Berlin is still the previous day in UTC, which is the day an
    // ISO-string conversion would show.
    expect(toDateTimeLocal(new Date("2026-10-04T22:30:00Z"))).toBe("2026-10-05T00:30");
  });
});

describe("fromDateTimeLocal", () => {
  it("reads the value as local time", () => {
    expect(fromDateTimeLocal("2026-07-01T14:00")?.toISOString()).toBe("2026-07-01T12:00:00.000Z");
    expect(fromDateTimeLocal("2026-01-15T14:00")?.toISOString()).toBe("2026-01-15T13:00:00.000Z");
  });

  it("accepts seconds, which some browsers add with a step", () => {
    expect(fromDateTimeLocal("2026-07-01T14:00:30")?.toISOString()).toBe("2026-07-01T12:00:30.000Z");
  });

  it("refuses empty, malformed and impossible values", () => {
    expect(fromDateTimeLocal("")).toBeNull();
    expect(fromDateTimeLocal("2026-07-01")).toBeNull();
    expect(fromDateTimeLocal("2026-07-01 14:00")).toBeNull();
    expect(fromDateTimeLocal("2026-02-31T10:00")).toBeNull();
  });

  it("round-trips through toDateTimeLocal across the year", () => {
    for (let month = 0; month < 12; month++) {
      const value = `2026-${String(month + 1).padStart(2, "0")}-15T09:30`;
      expect(toDateTimeLocal(fromDateTimeLocal(value)!)).toBe(value);
    }
  });

  it("round-trips on the days the clocks change", () => {
    // 29 March 2026, clocks go forward at 02:00; 25 October 2026, back at 03:00.
    for (const value of ["2026-03-29T01:30", "2026-03-29T03:30", "2026-10-25T01:30", "2026-10-25T04:00"]) {
      expect(toDateTimeLocal(fromDateTimeLocal(value)!)).toBe(value);
    }
    // A lecture across the spring change is an hour shorter in real time.
    const start = fromDateTimeLocal("2026-03-29T01:00")!;
    const end = fromDateTimeLocal("2026-03-29T04:00")!;
    expect(end.getTime() - start.getTime()).toBe(2 * 60 * 60 * 1000);
  });

  it("resolves a time the spring change skips to an hour later", () => {
    const skipped = fromDateTimeLocal("2026-03-29T02:30")!;
    expect(skipped.toISOString()).toBe("2026-03-29T01:30:00.000Z");
    expect(toDateTimeLocal(skipped)).toBe("2026-03-29T03:30");
  });

  it("resolves a time the autumn change repeats to one of its two instants", () => {
    const repeated = fromDateTimeLocal("2026-10-25T02:30")!;
    expect(["2026-10-25T00:30:00.000Z", "2026-10-25T01:30:00.000Z"]).toContain(repeated.toISOString());
    expect(toDateTimeLocal(repeated)).toBe("2026-10-25T02:30");
  });
});
