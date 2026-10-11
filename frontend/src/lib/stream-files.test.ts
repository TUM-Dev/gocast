import { describe, expect, it } from "vitest";

import { streamFileUrl } from "./stream-files";

describe("streamFileUrl", () => {
  it("addresses the file through its stream", () => {
    expect(streamFileUrl(7, 42)).toBe("/api/v2/streams/7/files/42");
  });

  it("asks for a download", () => {
    expect(streamFileUrl(7, 42, { download: true })).toBe("/api/v2/streams/7/files/42?download=1");
  });
});
