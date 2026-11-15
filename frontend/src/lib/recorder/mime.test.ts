import { describe, expect, it } from "vitest";

import { pickMimeType } from "./mime";

describe("pickMimeType", () => {
  it("prefers WebM, whose chunks come when asked for", () => {
    expect(pickMimeType(() => true)).toBe("video/webm;codecs=vp9,opus");
  });

  it("falls back to MP4, as Safari needs", () => {
    expect(pickMimeType((t) => t.startsWith("video/mp4"))).toBe("video/mp4;codecs=avc1,mp4a.40.2");
  });

  it("is null when the browser records nothing", () => {
    expect(pickMimeType(() => false)).toBeNull();
  });
});
