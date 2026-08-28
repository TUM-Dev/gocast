import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api";
import { useAuthStore } from "./auth";

/**
 * The store is what every page asks "who is signed in", and both the application shell
 * and the view it renders ask on mount. What matters is that asking twice costs one
 * request.
 */

const { fetchCurrentUser } = vi.hoisted(() => ({ fetchCurrentUser: vi.fn() }));

vi.mock("@/lib/settings", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/settings")>()),
  fetchCurrentUser,
}));

const user = { id: 1, name: "Hansi" };

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  fetchCurrentUser.mockResolvedValue(user);
});

describe("the auth store", () => {
  it("shares one request between callers that ask at the same time", async () => {
    const auth = useAuthStore();

    const [a, b] = await Promise.all([auth.load(), auth.load()]);

    expect(fetchCurrentUser).toHaveBeenCalledTimes(1);
    expect(a).toBe(b);
  });

  it("does not ask again once the user is loaded", async () => {
    const auth = useAuthStore();

    await auth.load();
    await auth.load();

    expect(fetchCurrentUser).toHaveBeenCalledTimes(1);
  });

  it("asks again when a caller forces a reload", async () => {
    const auth = useAuthStore();

    await auth.load();
    await auth.load(true);

    expect(fetchCurrentUser).toHaveBeenCalledTimes(2);
  });

  it("recovers after a failed load rather than caching the failure", async () => {
    const auth = useAuthStore();
    fetchCurrentUser.mockRejectedValueOnce(new Error("network"));

    await expect(auth.load()).rejects.toThrow("network");
    await expect(auth.load(true)).resolves.toEqual(user);
  });

  it("stays unloaded when the request fails for a reason other than a 401", async () => {
    // A 500 or a dropped connection leaves the question open. Marking the store
    // loaded there strands a signed-in user with a Login button for the life of the
    // page, because load() short-circuits on `loaded` and nothing asks again.
    const auth = useAuthStore();
    fetchCurrentUser.mockRejectedValueOnce(new Error("network down"));

    await expect(auth.load()).rejects.toThrow("network down");
    expect(auth.loaded).toBe(false);

    fetchCurrentUser.mockResolvedValueOnce(user);
    await expect(auth.load()).resolves.toEqual(user);
    expect(auth.loaded).toBe(true);
  });

  it("treats a 401 as a conclusive answer and does not ask again", async () => {
    const auth = useAuthStore();
    fetchCurrentUser.mockRejectedValueOnce(new ApiError(401, "no session"));

    await expect(auth.load()).resolves.toBeNull();
    expect(auth.loaded).toBe(true);

    await auth.load();
    expect(fetchCurrentUser).toHaveBeenCalledTimes(1);
  });
});
