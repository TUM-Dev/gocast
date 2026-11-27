import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api";
import { fetchLoginOptions, requestPasswordReset } from "./login";

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

afterEach(() => vi.restoreAllMocks());

describe("fetchLoginOptions", () => {
  it("reads the identity-provider fields from the public route", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(json({ useSaml: true, idpName: "TUM Login", idpColor: "#123456" }));

    await expect(fetchLoginOptions()).resolves.toEqual({
      useSaml: true,
      idpName: "TUM Login",
      idpColor: "#123456",
    });

    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toBe("/api/v2/login-options");
    expect(init?.method).toBe("GET");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("falls back to the default colour when the config sets none", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      json({ useSaml: false, idpName: "", idpColor: "" }),
    );

    await expect(fetchLoginOptions()).resolves.toEqual({
      useSaml: false,
      idpName: "",
      idpColor: "#3070B3",
    });
  });

  it("still defaults the colour when protojson leaves the field out", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(json({ useSaml: false }));

    expect((await fetchLoginOptions()).idpColor).toBe("#3070B3");
  });

  it("passes a failed request on", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(json({ code: 13, message: "down" }, 500));

    await expect(fetchLoginOptions()).rejects.toMatchObject({ status: 500, message: "down" });
  });
});

describe("requestPasswordReset", () => {
  it("posts the address to the public reset route", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(json({}));

    await expect(requestPasswordReset("erika@example.org")).resolves.toBeUndefined();

    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toBe("/api/v2/users/reset-password");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(String(init?.body))).toEqual({ email: "erika@example.org" });
  });

  it("passes a failed request on", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      json({ code: 8, message: "rate limited" }, 429),
    );

    await expect(requestPasswordReset("erika@example.org")).rejects.toBeInstanceOf(ApiError);
  });
});
