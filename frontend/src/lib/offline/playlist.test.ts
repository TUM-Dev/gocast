import { describe, expect, it } from "vitest";

import { hexToBytes, localPlaylist, parsePackagePlaylist, sequenceIV } from "./playlist";

const BASE = "https://edge.example/vod/offline/9/playlist.m3u8?jwt=abc";

// What ffmpeg writes with -hls_key_info_file and no IV line, after the edge appended
// the token to every segment.
const PACKAGE = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-KEY:METHOD=AES-128,URI="gocast-offline:9"
#EXTINF:10.000000,
00000.ts?jwt=abc
#EXTINF:4.200000,
00001.ts?jwt=abc
#EXT-X-ENDLIST
`;

describe("parsePackagePlaylist", () => {
  it("reads the segments, with the sequence number as IV", () => {
    const p = parsePackagePlaylist(PACKAGE, BASE);
    expect(p.targetDuration).toBe(10);
    expect(p.segments).toEqual([
      { url: "https://edge.example/vod/offline/9/00000.ts?jwt=abc", duration: 10, iv: sequenceIV(0) },
      { url: "https://edge.example/vod/offline/9/00001.ts?jwt=abc", duration: 4.2, iv: sequenceIV(1) },
    ]);
  });

  it("takes an explicit IV over the sequence number", () => {
    const p = parsePackagePlaylist(PACKAGE.replace('URI="gocast-offline:9"', 'URI="k",IV=0x0000000000000000000000000000ABCD'), BASE);
    expect(p.segments[1].iv).toBe("0000000000000000000000000000abcd");
  });

  it("refuses a package that is not encrypted", () => {
    expect(() => parsePackagePlaylist(PACKAGE.replace(/#EXT-X-KEY.*\n/, ""), BASE)).toThrow("not encrypted");
  });

  it("refuses what is not a playlist", () => {
    expect(() => parsePackagePlaylist("<html>", BASE)).toThrow();
  });
});

describe("sequenceIV", () => {
  it("is the sequence number as a 128 bit big-endian integer", () => {
    expect(sequenceIV(258)).toBe("00000000000000000000000000000102");
    expect(Array.from(hexToBytes(sequenceIV(258)).slice(-2))).toEqual([1, 2]);
  });
});

describe("localPlaylist", () => {
  it("names the service worker's URLs and has no key", () => {
    const text = localPlaylist("/offline-media/7", 10, [10, 4.2]);
    expect(text).not.toContain("EXT-X-KEY");
    expect(text).toContain("#EXTINF:4.200,\n/offline-media/7/1.ts");
    expect(text.trim().endsWith("#EXT-X-ENDLIST")).toBe(true);
  });
});
