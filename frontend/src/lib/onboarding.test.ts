import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api";
import { createFirstUser, firstUserErrorMessage, firstUserProblems } from "./onboarding";

afterEach(() => vi.restoreAllMocks());

describe("createFirstUser", () => {
  it("posts the account as JSON to the first-user route", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));

    await createFirstUser({ name: "Erika Mustermann", email: "erika@example.org", password: "correct horse" });

    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain("/users/first");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(String(init?.body))).toEqual({
      name: "Erika Mustermann",
      email: "erika@example.org",
      password: "correct horse",
    });
  });
});

describe("firstUserProblems", () => {
  it("wants a full name, an email address and eight characters", () => {
    expect(firstUserProblems({ name: "Erika", email: "erika", password: "short" })).toEqual({
      name: "Please enter your first and last name.",
      email: "Please enter a valid email address.",
      password: "The password must be at least 8 characters long.",
    });
    expect(firstUserProblems({ name: " Erika Mustermann ", email: "erika@example.org", password: "correct horse" })).toEqual({});
  });
});

describe("firstUserErrorMessage", () => {
  it("explains a set-up deployment, repeats validation and hides the rest", () => {
    expect(firstUserErrorMessage(new ApiError(403, "already has users"))).toContain("already has users");
    expect(firstUserErrorMessage(new ApiError(400, "name is required"))).toBe("name is required");
    expect(firstUserErrorMessage(new Error("boom"))).toBe("Something went wrong. Please try again.");
  });
});
