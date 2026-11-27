/**
 * The HLS playlists of offline lectures: reading the package a runner made (encrypted
 * with HLS AES-128), and writing the plain one the service worker hands the player.
 * Shared by the page and the service worker, so it uses neither's globals.
 */

export interface PackageSegment {
  /** The segment's URL as the playlist names it, resolved against the playlist. */
  url: string;
  duration: number;
  /** The AES-CBC IV, 32 hex digits. */
  iv: string;
}

export interface PackagePlaylist {
  targetDuration: number;
  segments: PackageSegment[];
}

/**
 * Reads a package's playlist. Every segment has to be encrypted with AES-128: a package
 * that is not is refused rather than stored in the clear.
 */
export function parsePackagePlaylist(text: string, baseUrl: string): PackagePlaylist {
  const lines = text.split(/\r?\n/).map((l) => l.trim());
  if (lines[0] !== "#EXTM3U") throw new Error("not an HLS playlist");

  let targetDuration = 0;
  let sequence = 0;
  let duration: number | null = null;
  let method = "NONE";
  let explicitIV: string | null = null;
  const segments: PackageSegment[] = [];

  for (const line of lines) {
    if (line.startsWith("#EXT-X-TARGETDURATION:")) {
      targetDuration = Number(line.slice("#EXT-X-TARGETDURATION:".length));
    } else if (line.startsWith("#EXT-X-MEDIA-SEQUENCE:")) {
      sequence = Number(line.slice("#EXT-X-MEDIA-SEQUENCE:".length));
    } else if (line.startsWith("#EXT-X-KEY:")) {
      const attrs = parseAttributes(line.slice("#EXT-X-KEY:".length));
      method = attrs.METHOD ?? "NONE";
      explicitIV = attrs.IV ? normalizeIV(attrs.IV) : null;
    } else if (line.startsWith("#EXTINF:")) {
      duration = Number.parseFloat(line.slice("#EXTINF:".length));
    } else if (line && !line.startsWith("#")) {
      if (duration === null || Number.isNaN(duration)) throw new Error("segment without a duration");
      if (method !== "AES-128") throw new Error("the package is not encrypted");
      segments.push({ url: new URL(line, baseUrl).href, duration, iv: explicitIV ?? sequenceIV(sequence) });
      sequence++;
      duration = null;
    }
  }
  if (segments.length === 0) throw new Error("the package has no segments");
  return { targetDuration: targetDuration || Math.ceil(Math.max(...segments.map((s) => s.duration))), segments };
}

/** The IV HLS uses when the key names none: the media sequence number, big-endian. */
export function sequenceIV(sequence: number): string {
  return sequence.toString(16).padStart(32, "0");
}

function normalizeIV(iv: string): string {
  const hex = iv.replace(/^0x/i, "").toLowerCase();
  if (!/^[0-9a-f]{1,32}$/.test(hex)) throw new Error("invalid IV");
  return hex.padStart(32, "0");
}

export function hexToBytes(hex: string): Uint8Array<ArrayBuffer> {
  const out = new Uint8Array(hex.length / 2);
  for (let i = 0; i < out.length; i++) out[i] = Number.parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  return out;
}

/** Splits `A=B,C="D,E"` into {A: "B", C: "D,E"}. */
function parseAttributes(list: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const m of list.matchAll(/([A-Z0-9-]+)=("[^"]*"|[^,]*)/g)) {
    out[m[1]] = m[2].replace(/^"|"$/g, "");
  }
  return out;
}

/**
 * The playlist the player gets for a saved lecture: no key line, since the service
 * worker decrypts each segment before the player sees it, and segment URLs the service
 * worker answers from the device.
 */
export function localPlaylist(base: string, targetDuration: number, durations: readonly number[]): string {
  const lines = [
    "#EXTM3U",
    "#EXT-X-VERSION:3",
    `#EXT-X-TARGETDURATION:${Math.ceil(targetDuration)}`,
    "#EXT-X-MEDIA-SEQUENCE:0",
    "#EXT-X-PLAYLIST-TYPE:VOD",
  ];
  durations.forEach((d, i) => lines.push(`#EXTINF:${d.toFixed(3)},`, `${base}/${i}.ts`));
  lines.push("#EXT-X-ENDLIST", "");
  return lines.join("\n");
}
