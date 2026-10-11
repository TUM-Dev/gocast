import { describe, expect, it, vi } from "vitest";

import { setSubtitleTracks } from "./tracks";

describe("setSubtitleTracks", () => {
  it("adds a subtitles track per file and removes exactly those", () => {
    const added: unknown[] = [];
    const host = {
      addRemoteTextTrack: vi.fn((o: object) => {
        const track = { o };
        added.push(track);
        return { track };
      }),
      removeRemoteTextTrack: vi.fn(),
    };
    const undo = setSubtitleTracks(host, [
      { src: "/de.vtt", lang: "de", label: "Deutsch", default: true },
      { src: "/en.vtt", lang: "en" },
    ]);
    expect(host.addRemoteTextTrack).toHaveBeenNthCalledWith(
      1,
      { kind: "subtitles", src: "/de.vtt", srclang: "de", label: "Deutsch", default: true },
      false,
    );
    expect(host.addRemoteTextTrack).toHaveBeenNthCalledWith(
      2,
      { kind: "subtitles", src: "/en.vtt", srclang: "en", label: "en", default: undefined },
      false,
    );
    undo();
    expect(host.removeRemoteTextTrack.mock.calls.map((c) => c[0])).toEqual(added);
  });
});
