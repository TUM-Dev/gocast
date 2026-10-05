import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api";
import {
  checkPasswordResetKey,
  passwordProblem,
  setPasswordByResetKey,
  setPasswordErrorMessage,
} from "./password-reset";

const okResponse = () => new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } });
const errorResponse = (status: number, message: string) =>
  new Response(JSON.stringify({ code: 5, message }), { status, headers: { "Content-Type": "application/json" } });

afterEach(() => vi.restoreAllMocks());

describe("checkPasswordResetKey", () => {
  it("is true while the key opens the page and false once it does not", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(okResponse());
    expect(await checkPasswordResetKey("abc/def")).toBe(true);
    expect(String(fetchMock.mock.calls[0][0])).toContain("/users/password-reset/abc%2Fdef");

    fetchMock.mockResolvedValueOnce(errorResponse(404, "this link is not valid any more"));
    expect(await checkPasswordResetKey("used")).toBe(false);
  });

  it("passes any other failure on", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(errorResponse(500, "down"));
    await expect(checkPasswordResetKey("key")).rejects.toBeInstanceOf(ApiError);
  });
});

describe("setPasswordByResetKey", () => {
  it("posts the password as JSON to the key's path", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(okResponse());
    await setPasswordByResetKey("key", "correct horse");

    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain("/users/password-reset/key");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(String(init?.body))).toEqual({ password: "correct horse" });
  });
});

describe("passwordProblem", () => {
  it("wants eight characters and a matching confirmation", () => {
    expect(passwordProblem("short", "short")).toContain("at least 8");
    expect(passwordProblem("correct horse", "correct hors")).toBe("The passwords do not match.");
    expect(passwordProblem("correct horse", "correct horse")).toBe("");
  });
});

describe("setPasswordErrorMessage", () => {
  it("explains a dead link, repeats a validation message and hides the rest", () => {
    expect(setPasswordErrorMessage(new ApiError(404, "nope"))).toContain("not valid any more");
    expect(setPasswordErrorMessage(new ApiError(400, "the password must be at least 8 characters long"))).toBe(
      "the password must be at least 8 characters long",
    );
    expect(setPasswordErrorMessage(new Error("boom"))).toBe("Something went wrong. Please try again.");
  });
});
