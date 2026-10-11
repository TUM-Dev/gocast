/** The transcript: WebVTT subtitles read as a list of cues the panel can follow. */

import { ApiError } from "./api";

export interface TranscriptCue {
  /** Seconds. */
  start: number;
  end: number;
  text: string;
}

/** "00:01:02.500", "01:02.500" or "1:02,500" to seconds; NaN if it is none of those. */
function parseStamp(stamp: string): number {
  const m = /^(?:(\d+):)?(\d{1,2}):(\d{2})[.,](\d{1,3})$/.exec(stamp.trim());
  if (!m) return NaN;
  const [, h, min, s, frac] = m;
  return Number(h ?? 0) * 3600 + Number(min) * 60 + Number(s) + Number(frac.padEnd(3, "0")) / 1000;
}

/**
 * Cues of a WebVTT file. Header, NOTE and STYLE blocks and cue identifiers are
 * skipped; markup inside a cue (<v Name>, <c>, <i>) is dropped and its lines joined
 * with a space, since the panel wraps text itself.
 */
export function parseVtt(vtt: string): TranscriptCue[] {
  const cues: TranscriptCue[] = [];
  for (const block of vtt.replace(/\r\n?/g, "\n").split(/\n{2,}/)) {
    const lines = block.split("\n");
    const at = lines.findIndex((l) => l.includes("-->"));
    if (at < 0 || lines[0].startsWith("NOTE")) continue;
    const [from, rest = ""] = lines[at].split("-->");
    const start = parseStamp(from);
    const end = parseStamp(rest.trim().split(/\s+/)[0] ?? "");
    if (Number.isNaN(start) || Number.isNaN(end)) continue;
    const text = lines
      .slice(at + 1)
      .join(" ")
      .replace(/<[^>]*>/g, "")
      .replace(/\s+/g, " ")
      .trim();
    if (text) cues.push({ start, end, text });
  }
  return cues;
}

/** Index of the cue showing at `time`, or -1 in a gap. Cues are in time order. */
export function activeCueIndex(cues: TranscriptCue[], time: number): number {
  let lo = 0;
  let hi = cues.length - 1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    if (cues[mid].start > time) hi = mid - 1;
    else if (cues[mid].end < time) lo = mid + 1;
    else return mid;
  }
  return -1;
}

/** What "Export transcript" saves: the cue texts, a blank line between them. */
export function transcriptText(cues: TranscriptCue[]): string {
  return cues.map((c) => `${c.text}\n\n`).join("");
}

/**
 * One language's subtitles. The RPC answers with the raw file (text/vtt), not JSON,
 * so it is read as text. Sent on the session cookie like the thumbnails are.
 */
export async function fetchTranscript(slug: string, streamId: number, lang: string): Promise<TranscriptCue[]> {
  const res = await fetch(
    `/api/v2/streams/${encodeURIComponent(slug)}/${streamId}/subtitles/${encodeURIComponent(lang)}`,
    { credentials: "same-origin" },
  );
  if (!res.ok) throw new ApiError(res.status, res.statusText || "could not load the transcript");
  return parseVtt(await res.text());
}

/** Hands the text to the browser as a file; the one place the transcript touches the DOM. */
export function saveTextFile(text: string, filename: string): void {
  const url = URL.createObjectURL(new Blob([text], { type: "text/plain" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
