import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { createCourse, sanitizeSlug, searchTumOnlineCourses } from "./create-course";

let fetchMock: ReturnType<typeof vi.fn>;

/** Answers the token request, then the given body for everything else. */
function respondWith(body: unknown, status = 200): void {
  fetchMock.mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(
        new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 }),
      );
    }
    return Promise.resolve(new Response(JSON.stringify(body), { status }));
  });
}

const lastCall = () => fetchMock.mock.calls.at(-1) as [string, RequestInit];

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("searchTumOnlineCourses", () => {
  it("escapes the query and reads the matches", async () => {
    respondWith({ courses: [{ tumOnlineId: "950", name: "Brauereiwesen", year: 2026, term: "S" }] });

    const found = await searchTumOnlineCourses("bier & brot");

    expect(lastCall()[0]).toBe("/api/v2/tumonline-courses?q=bier%20%26%20brot");
    expect(found).toEqual([{ tumOnlineId: "950", name: "Brauereiwesen", year: 2026, term: "S" }]);
  });

  it("reads no matches as an empty list", async () => {
    // protojson leaves out an empty repeated field.
    respondWith({});

    expect(await searchTumOnlineCourses("x")).toEqual([]);
  });
});

describe("createCourse", () => {
  it("posts the course and answers its ID", async () => {
    respondWith({ courseId: 42 });

    const id = await createCourse({
      name: "Brauereiwesen",
      slug: "brau",
      year: 2026,
      term: "W",
      tumOnlineId: "",
      language: "de",
    });

    expect(id).toBe(42);
    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toMatchObject({
      name: "Brauereiwesen",
      slug: "brau",
      year: 2026,
      term: "W",
      language: "de",
    });
  });
});

describe("sanitizeSlug", () => {
  it("keeps what the old form kept", () => {
    expect(sanitizeSlug("Ei-Di_2 ä/ß!")).toBe("Ei-Di_2");
  });
});
