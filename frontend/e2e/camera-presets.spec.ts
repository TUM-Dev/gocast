import { type APIRequestContext, expect, test } from "@playwright/test";

import { apiAs, bearerToken } from "./helpers";
import { users } from "./seed";

/**
 * Switching a live stream's camera preset, the watch page's one call into the
 * cameras.
 *
 * The dump has no camera and no presets, so nothing here reaches hardware; what it
 * proves is everything in front of the camera. The fixture rows it leans on:
 *   - stream 7 is Spieleentwicklung's (course 2), live, in lecture hall 1;
 *   - stream 1 is Einführung Brauereiwesen's (course 1), not live;
 *   - lecture hall 1 has no presets;
 *   - prof2 administers courses 1 and 2, prof1 courses 1 and 3.
 * Every request is refused before anything is written, so the file needs no reload.
 */

const switchPath = (courseId: number, streamId: number, presetId = 1) =>
  `/api/v2/courses/${courseId}/streams/${streamId}/presets/${presetId}/switch`;

async function post(context: APIRequestContext, path: string) {
  const token = await bearerToken(context);
  return context.post(path, token ? { headers: { Authorization: `Bearer ${token}` } } : {});
}

test.describe("switching a camera preset", () => {
  test("reaches the stream's hall for an administrator of its course", async ({ playwright }) => {
    // Past the policy and every stream check: the only thing missing is the preset.
    const response = await post(await apiAs(playwright, users.prof2), switchPath(2, 7));

    expect(response.status()).toBe(404);
    expect((await response.json()).message).toBe("can not find preset");
  });

  test("accepts the session cookie the watch page calls it with", async ({ playwright }) => {
    const context = await apiAs(playwright, users.prof2);
    const response = await context.post(switchPath(2, 7));

    expect(response.status()).toBe(404);
    expect((await response.json()).message).toBe("can not find preset");
  });

  test("is refused to a lecturer who does not administer the course", async ({ playwright }) => {
    const response = await post(await apiAs(playwright, users.prof1), switchPath(2, 7));

    expect(response.status()).toBe(404);
    expect((await response.json()).message).toBe("no such course");
  });

  test("cannot reach another course's stream through one's own course", async ({ playwright }) => {
    // prof1 administers course 1, but stream 7 is course 2's.
    const response = await post(await apiAs(playwright, users.prof1), switchPath(1, 7));

    expect(response.status()).toBe(404);
    expect((await response.json()).message).toBe("no such stream");
  });

  test("refuses a stream that is not live", async ({ playwright }) => {
    const response = await post(await apiAs(playwright, users.prof1), switchPath(1, 1));

    expect(response.status()).toBe(400);
    expect((await response.json()).message).toBe("the stream is not live");
  });

  test("is refused to a student", async ({ playwright }) => {
    const response = await post(await apiAs(playwright, users.studi1), switchPath(2, 7));

    expect(response.status()).toBe(404);
  });

  test("is refused to an anonymous caller", async ({ playwright }) => {
    const response = await post(await apiAs(playwright), switchPath(2, 7));

    expect(response.status()).toBe(401);
  });

  test("is no longer served by the v1 API", async ({ playwright }) => {
    const context = await apiAs(playwright, users.prof2);
    const response = await context.post("/api/course/2/switchPreset/1/1/7");

    // v1 answered this with a JSON 404 of its own -- the preset is missing -- so the
    // status alone cannot tell the route is gone. The router's not-found page can.
    expect(response.status()).toBe(404);
    expect(await response.text()).toContain("This page does not exist.");
  });
});
