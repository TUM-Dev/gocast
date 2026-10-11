import { afterEach, describe, expect, it, vi } from "vitest";

import { routeFetch } from "@/components/watch/panels/testing";
import {
  activeCueIndex,
  fetchTranscript,
  parseVtt,
  saveTextFile,
  transcriptText,
} from "./watch-panels-transcript";

afterEach(() => vi.unstubAllGlobals());

const VTT = `WEBVTT

NOTE a comment
that spans lines

1
00:00:01.000 --> 00:00:03.500 align:start
Hello <i>everyone</i>

00:03.500 --> 00:05.000
<v Prof>second
line</v>

00:00:09,250 --> 00:00:10,000
after a gap

00:00:11.000 --> 00:00:12.000

`;

describe("parseVtt", () => {
  it("reads cues, dropping notes, identifiers, settings, markup and empty cues", () => {
    expect(parseVtt(VTT)).toEqual([
      { start: 1, end: 3.5, text: "Hello everyone" },
      { start: 3.5, end: 5, text: "second line" },
      { start: 9.25, end: 10, text: "after a gap" },
    ]);
  });

  it("reads hours and CRLF line endings", () => {
    expect(parseVtt("WEBVTT\r\n\r\n01:00:00.000 --> 01:00:01.000\r\nlate\r\n")).toEqual([
      { start: 3600, end: 3601, text: "late" },
    ]);
  });

  it("ignores a block with an unreadable time", () => {
    expect(parseVtt("WEBVTT\n\nsoon --> later\nx")).toEqual([]);
  });
});

describe("activeCueIndex", () => {
  const cues = parseVtt(VTT);
  it("finds the cue showing, and -1 in the gaps and outside", () => {
    expect(activeCueIndex(cues, 0.5)).toBe(-1);
    expect(activeCueIndex(cues, 1)).toBe(0);
    expect(activeCueIndex(cues, 4)).toBe(1);
    expect(activeCueIndex(cues, 7)).toBe(-1);
    expect(activeCueIndex(cues, 9.5)).toBe(2);
    expect(activeCueIndex(cues, 99)).toBe(-1);
    expect(activeCueIndex([], 1)).toBe(-1);
  });
});

describe("transcriptText", () => {
  it("is the cue texts with a blank line after each", () => {
    expect(transcriptText(parseVtt(VTT))).toBe("Hello everyone\n\nsecond line\n\nafter a gap\n\n");
  });
});

describe("fetchTranscript", () => {
  it("reads the language's file as text", async () => {
    const { calls } = routeFetch([{ path: /subtitles\/en$/, text: "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nhi" }]);
    expect(await fetchTranscript("ws24", 5, "en")).toEqual([{ start: 1, end: 2, text: "hi" }]);
    expect(calls[0].url).toBe("/api/v2/streams/ws24/5/subtitles/en");
  });

  it("throws with the status when the language is missing", async () => {
    routeFetch([]);
    await expect(fetchTranscript("ws24", 5, "de")).rejects.toMatchObject({ status: 404 });
  });
});

describe("saveTextFile", () => {
  it("clicks a download link for the text and cleans up", () => {
    const create = vi.fn(() => "blob:x");
    const revoke = vi.fn();
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: create, revokeObjectURL: revoke }));
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    saveTextFile("text", "transcript.txt");
    expect(create).toHaveBeenCalled();
    expect(click).toHaveBeenCalled();
    expect(revoke).toHaveBeenCalledWith("blob:x");
    expect(document.querySelector("a[download]")).toBeNull();
    click.mockRestore();
  });
});
