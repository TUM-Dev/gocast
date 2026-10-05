/**
 * Conversions between Date and the value of an `<input type="datetime-local">`.
 *
 * The input's value is a wall-clock time with no zone, `YYYY-MM-DDTHH:MM`, and the
 * browser shows it as is. So both directions go through the local getters and the
 * local Date constructor: `toISOString()` would shift the shown time by the UTC
 * offset, and `new Date("YYYY-MM-DDTHH:MM")` is only local by a spec rule that older
 * engines got wrong.
 */

const pad = (n: number) => String(n).padStart(2, "0");

/** The input value showing `date` in local time, to the minute. */
export function toDateTimeLocal(date: Date): string {
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    `T${pad(date.getHours())}:${pad(date.getMinutes())}`
  );
}

const LOCAL = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/;

/**
 * The instant an input value names in local time, or null for an empty or malformed
 * value. A time skipped by a daylight-saving change (02:30 on the spring-forward
 * night) comes back as the instant the clock read an hour later, as the Date
 * constructor resolves it; one that happens twice comes back as its first occurrence.
 */
export function fromDateTimeLocal(value: string): Date | null {
  const m = LOCAL.exec(value);
  if (!m) return null;
  const [year, month, day, hours, minutes, seconds] = m.slice(1).map((s) => Number(s ?? 0));
  const date = new Date(year, month - 1, day, hours, minutes, seconds);
  // Rejects 2026-02-31 and friends rather than rolling them into March.
  if (date.getMonth() !== month - 1 || date.getDate() !== day) return null;
  return date;
}
