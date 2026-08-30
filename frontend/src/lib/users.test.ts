import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { Role, roleLabel, searchUsers, searchable, updateUserRole } from "./users";

/**
 * The parts of the users page that are decisions rather than plumbing: when a search
 * is worth sending, and what the client does with a role it does not recognise.
 */

let fetchMock: ReturnType<typeof vi.fn>;

function respondWith(body: unknown): void {
  fetchMock.mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(
        new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 }),
      );
    }
    return Promise.resolve(new Response(JSON.stringify(body), { status: 200 }));
  });
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("searchable", () => {
  // Mirrors the server, which refuses a short query outright. Asking anyway would put
  // a 400 on screen for every one- and two-letter prefix somebody types.
  it("wants three characters when there is nothing else to go on", () => {
    expect(searchable("ab")).toBe(false);
    expect(searchable("abc")).toBe(true);
  });

  it("ignores surrounding whitespace", () => {
    expect(searchable("  ab  ")).toBe(false);
  });

  // "Every lecturer" is a search with no query at all.
  it("accepts a role on its own", () => {
    expect(searchable("", Role.lecturer)).toBe(true);
    expect(searchable("a", Role.lecturer)).toBe(true);
  });
});

describe("searchUsers", () => {
  it("sends the query and the role", async () => {
    respondWith({ users: [] });

    await searchUsers("prof", Role.lecturer);

    const [url] = fetchMock.mock.calls.at(-1) as [string];
    expect(url).toContain("/admin/users/search?");
    expect(url).toContain("query=prof");
    expect(url).toContain("role=2");
  });

  it("omits the role when searching every account", async () => {
    respondWith({ users: [] });

    await searchUsers("prof");

    const [url] = fetchMock.mock.calls.at(-1) as [string];
    expect(url).not.toContain("role=");
  });

  it("reads a search matching nobody as an empty list", async () => {
    // protojson omits an empty repeated field, so the body is `{}`.
    respondWith({});

    await expect(searchUsers("nobody")).resolves.toEqual([]);
  });
});

describe("roleLabel", () => {
  it.each([
    [Role.admin, "Admin"],
    [Role.lecturer, "Lecturer"],
    [Role.student, "Student"],
    // Someone invited to a single course, who has no account of their own. The old
    // page called every non-staff role "Generic" in one list and "Student" in another.
    [Role.generic, "Invited"],
  ])("names role %i", (role, expected) => {
    expect(roleLabel(role)).toBe(expected);
  });

  it("shows a role it does not know rather than guessing", () => {
    // Roles are the server's to define. Falling back to "Student" would tell an
    // administrator that an account has fewer rights than it does.
    expect(roleLabel(42)).toBe("Role 42");
  });
});

describe("updateUserRole", () => {
  it("patches the role of one account", async () => {
    respondWith({ id: 3, name: "Someone", email: "", lrzId: "", role: 2 });

    const updated = await updateUserRole(3, Role.lecturer);

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/users/3/role");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(String(init.body))).toMatchObject({ role: 2 });
    expect(updated.role).toBe(Role.lecturer);
  });
});
