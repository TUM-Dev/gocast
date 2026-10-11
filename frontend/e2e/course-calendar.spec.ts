import { expect, test } from "@playwright/test";

import { apiAs } from "./helpers";
import { courses } from "./seed";

/**
 * A course's calendar feed. Calendars were subscribed to the v1 address years ago and
 * keep polling it, so both addresses must answer, identically and without signing in
 * (a calendar app cannot).
 */
const { year, term } = courses.brauereiwesen;
const slug = "brauereiwesen";
const v2 = `/api/v2/courses/${year}/${term}/${slug}/events.ics`;
const v1 = `/api/download_ics/${year}/${term}/${slug}/events.ics`;

test.describe("a course's calendar feed", () => {
  test("is served at the v2 address to anyone", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    const response = await anonymous.get(v2);

    expect(response.status()).toBe(200);
    expect(response.headers()["content-type"]).toContain("text/calendar");
    expect(response.headers()["content-disposition"]).toContain(
      `${slug}${term}${year}.ics`,
    );
    const body = await response.text();
    expect(body).toContain("BEGIN:VCALENDAR");
    expect(body).toContain("BEGIN:VEVENT");
  });

  test("is still served in place at the address calendars subscribed to", async ({
    playwright,
  }) => {
    const anonymous = await apiAs(playwright);
    const [old, current] = await Promise.all([
      anonymous.get(v1, { maxRedirects: 0 }),
      anonymous.get(v2),
    ]);

    // Not a redirect: some calendar clients do not follow one.
    expect(old.status()).toBe(200);
    expect(await old.text()).toBe(await current.text());
  });

  test("answers 404 for a course that does not exist and 400 for a bad year", async ({
    playwright,
  }) => {
    const anonymous = await apiAs(playwright);

    expect(
      (
        await anonymous.get(
          `/api/v2/courses/${year}/${term}/no-such-course/events.ics`,
        )
      ).status(),
    ).toBe(404);
    expect(
      (
        await anonymous.get(`/api/v2/courses/abc/${term}/${slug}/events.ics`)
      ).status(),
    ).toBe(400);
  });
});
