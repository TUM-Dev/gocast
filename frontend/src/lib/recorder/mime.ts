/**
 * The format a MediaRecorder records in. WebM first, although the runner has to convert
 * it: Chrome's MP4 recorder ignores the timeslice and holds the video back, handing out
 * a kilobyte of header after seconds, so a crashed tab would lose most of what it
 * recorded. Its WebM recorder hands out every two seconds what they recorded. MP4 is
 * for Safari, which records nothing else.
 */
export const PREFERRED_MIME_TYPES = [
  "video/webm;codecs=vp9,opus",
  "video/webm;codecs=vp8,opus",
  "video/webm",
  "video/mp4;codecs=avc1,mp4a.40.2",
  "video/mp4",
] as const;

/** The first of PREFERRED_MIME_TYPES the browser records, or null when it records none. */
export function pickMimeType(isTypeSupported: (type: string) => boolean): string | null {
  return PREFERRED_MIME_TYPES.find((type) => isTypeSupported(type)) ?? null;
}
