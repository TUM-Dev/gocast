import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api";
import {
  MISSING_TOKEN_MESSAGE,
  courseTokenErrorMessage,
  fetchCourseByToken,
  optInCourseByToken,
  optOutCourseByToken,
  tokenFromQuery,
} from "./course-token";

const jsonResponse = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

afterEach(() => vi.restoreAllMocks());

describe("fetchCourseByToken", () => {
  it("posts the token in the body and reads the course back", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        jsonResponse(200, { id: 3, name: "Golang", slug: "godev", year: 2021, term: "W", optedOut: true }),
      );

    const course = await fetchCourseByToken("secret");

    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain("/courses/by-token");
    expect(String(url)).not.toContain("secret");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(String(init?.body))).toEqual({ token: "secret" });
    expect(course).toEqual({ id: 3, name: "Golang", slug: "godev", year: 2021, term: "W", optedOut: true });
  });
});

describe("opting in and out", () => {
  it("post to their own paths", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(() => Promise.resolve(jsonResponse(200, {})));
    await optInCourseByToken("secret");
    await optOutCourseByToken("secret");

    expect(String(fetchMock.mock.calls[0][0])).toContain("/courses/by-token/opt-in");
    expect(String(fetchMock.mock.calls[1][0])).toContain("/courses/by-token/opt-out");
  });
});

describe("tokenFromQuery", () => {
  it("takes a trimmed string and nothing else", () => {
    expect(tokenFromQuery({ token: " abc " })).toBe("abc");
    expect(tokenFromQuery({ token: ["a", "b"] })).toBe("");
    expect(tokenFromQuery({})).toBe("");
  });
});

describe("courseTokenErrorMessage", () => {
  it("names a missing or unknown token and hides the rest", () => {
    expect(courseTokenErrorMessage(new ApiError(400, "token is required"))).toBe(MISSING_TOKEN_MESSAGE);
    expect(courseTokenErrorMessage(new ApiError(404, "no course has this token"))).toContain("No course matches");
    expect(courseTokenErrorMessage(new Error("boom"))).toContain("Something went wrong");
  });
});
