import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { fetchCurrentUser, updateSetting, UserSettingType } from "./settings";

/**
 * Settings are not stored uniformly: each getter in model/user.go decides how to read
 * its value, so some are JSON and some are bare strings. Encoding a name as JSON once
 * shipped quotes into every page that displays it, which is what these tests pin down.
 *
 * Only `fetch` is stubbed, so the generated schemas do the real serialisation and the
 * assertions below describe the actual bytes on the wire.
 */

let fetchMock: ReturnType<typeof vi.fn>;

function tokenResponse(): Response {
  return new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 });
}

/** The body of the last non-token request. */
function lastRequestBody(): { userSettings: { type: string; value: string }[] } {
  const dataCalls = fetchMock.mock.calls.filter((c) => !String(c[0]).endsWith("/auth/token"));
  const init = dataCalls[dataCalls.length - 1][1] as RequestInit;
  return JSON.parse(String(init.body));
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn().mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(tokenResponse());
    }
    return Promise.resolve(new Response(JSON.stringify({ userSettings: [] }), { status: 200 }));
  });
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("updateSetting", () => {
  it("sends the preferred name unquoted", async () => {
    // GetPreferredName returns setting.Value verbatim, so JSON encoding here would
    // put literal quotes into the name everywhere it is displayed.
    await updateSetting(UserSettingType.PREFERRED_NAME, "Hansi");

    expect(lastRequestBody().userSettings[0].value).toBe("Hansi");
  });

  it.each([
    [UserSettingType.GREETING, "Servus"],
    [UserSettingType.LECTURE_VIEW, "Camera"],
  ])("sends bare-string setting %s unquoted", async (type, value) => {
    await updateSetting(type, value);

    expect(lastRequestBody().userSettings[0].value).toBe(value);
  });

  it("sends auto skip as JSON, because the getter unmarshals it", async () => {
    await updateSetting(UserSettingType.AUTO_SKIP, { enabled: true });

    expect(lastRequestBody().userSettings[0].value).toBe('{"enabled":true}');
  });

  it("sends custom speeds as a JSON array", async () => {
    await updateSetting(UserSettingType.USER_DEFINED_SPEEDS, [1.1, 2.5]);

    expect(lastRequestBody().userSettings[0].value).toBe("[1.1,2.5]");
  });

  it("sends the seeking time as a plain number", async () => {
    // Read back with strconv.Atoi, which would reject a quoted value.
    await updateSetting(UserSettingType.SEEKING_TIME, 30);

    expect(lastRequestBody().userSettings[0].value).toBe("30");
  });

  it("names the setting type on the wire", async () => {
    // Before the enum was completed in the proto these travelled as bare numbers.
    await updateSetting(UserSettingType.LECTURE_VIEW, "Split");

    expect(lastRequestBody().userSettings[0].type).toBe("LECTURE_VIEW");
  });
});

describe("fetchCurrentUser", () => {
  function respondWithSettings(settings: { type: string; value: string }[]): void {
    fetchMock.mockImplementation((url: string) => {
      if (String(url).endsWith("/auth/token")) {
        return Promise.resolve(tokenResponse());
      }
      return Promise.resolve(
        new Response(
          JSON.stringify({
            user: { id: 1, name: "Fallback Name", email: "a@b.c", role: 4, settings },
          }),
          { status: 200 },
        ),
      );
    });
  }

  it("reads every setting type", async () => {
    respondWithSettings([
      { type: "PREFERRED_NAME", value: "Hansi" },
      { type: "GREETING", value: "Servus" },
      { type: "LECTURE_VIEW", value: "Camera" },
      { type: "SEEKING_TIME", value: "30" },
      { type: "AUTO_SKIP", value: '{"enabled":true}' },
      { type: "USER_DEFINED_SPEEDS", value: "[1.1]" },
    ]);

    const user = await fetchCurrentUser();

    expect(user.settings).toMatchObject({
      preferredName: "Hansi",
      greeting: "Servus",
      lectureView: "Camera",
      seekingTime: 30,
      autoSkip: true,
      customSpeeds: [1.1],
    });
  });

  it("defaults playback speeds to the same list model/user.go uses", async () => {
    // The first toggle a user makes saves this whole array, so a mismatch here is
    // written to their account: speeds the player offered silently turn off, and any
    // entry missing here is removed as an option altogether.
    respondWithSettings([]);

    const user = await fetchCurrentUser();

    expect(user.settings.playbackSpeeds).toEqual([
      { speed: 0.25, enabled: false },
      { speed: 0.5, enabled: true },
      { speed: 0.75, enabled: true },
      { speed: 1, enabled: true },
      { speed: 1.25, enabled: true },
      { speed: 1.5, enabled: true },
      { speed: 1.75, enabled: true },
      { speed: 2, enabled: true },
      { speed: 2.5, enabled: false },
      { speed: 3, enabled: false },
      { speed: 3.5, enabled: false },
    ]);
  });

  it("gives each load its own copy of the defaults", async () => {
    // SettingsView toggles `entry.enabled` in place. Handing out the module-level
    // array would let one user's unsaved toggle persist into the next load, which is
    // exactly what the failed-save path reloads to undo.
    respondWithSettings([]);

    const first = await fetchCurrentUser();
    first.settings.playbackSpeeds[0].enabled = true;

    const second = await fetchCurrentUser();
    expect(second.settings.playbackSpeeds[0].enabled).toBe(false);
  });

  it("unwraps a name that was stored JSON-encoded", async () => {
    // Heals rows written before the encoding was fixed, rather than showing quotes.
    respondWithSettings([{ type: "PREFERRED_NAME", value: '"Hansi"' }]);

    const user = await fetchCurrentUser();

    expect(user.settings.preferredName).toBe("Hansi");
  });

  it("falls back to the values model/user.go defaults to", async () => {
    respondWithSettings([]);

    const user = await fetchCurrentUser();

    expect(user.settings).toMatchObject({
      preferredName: "Fallback Name",
      greeting: "Moin",
      lectureView: "Combined",
      seekingTime: 10,
      autoSkip: false,
      customSpeeds: [],
    });
  });
});
