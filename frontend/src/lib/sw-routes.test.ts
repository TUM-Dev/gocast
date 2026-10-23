import { describe, expect, it } from "vitest";

import { isAppNavigation, isBrandingFile } from "./sw-routes";

const at = (path: string) => new URL(path, "https://live.example.org");

describe("isAppNavigation", () => {
  it.each(["/", "/settings", "/w/eidi/1", "/login", "/admin/courses/3", "/publication"])(
    "lets the shell stand in for %s",
    (path) => expect(isAppNavigation(at(path))).toBe(true),
  );

  it.each(["/api/download/1", "/saml/out", "/saml", "/logout", "/public/x.pdf", "/offline-media/7/playlist.m3u8"])(
    "leaves %s to the server",
    (path) => expect(isAppNavigation(at(path))).toBe(false),
  );
});

describe("isBrandingFile", () => {
  it("matches the root branding files only", () => {
    expect(isBrandingFile(at("/manifest.json"))).toBe(true);
    expect(isBrandingFile(at("/icons-512.png"))).toBe(true);
    expect(isBrandingFile(at("/spa-assets/logo.svg"))).toBe(false);
  });
});
