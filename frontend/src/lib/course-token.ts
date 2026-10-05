/**
 * The two pages a lecturer reaches from the mail sent when their course is imported
 * from TUMOnline: /edit-course?token= to switch it on, /edit-course/opt-out?token= to
 * have it deleted. The token is the only credential; every call is public.
 */

import { fromJson } from "@bufbuild/protobuf";

import { CourseByTokenSchema } from "@/gen/server/apiv2_pb";
import { ApiError, apiFetchPublic } from "./api";

export interface CourseByToken {
  id: number;
  name: string;
  slug: string;
  year: number;
  term: string;
  /** Opted out earlier: soft-deleted, and opting in restores it. */
  optedOut: boolean;
}

function post<T>(path: string, token: string): Promise<T> {
  return apiFetchPublic<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
}

export async function fetchCourseByToken(token: string): Promise<CourseByToken> {
  const json = await post<unknown>("/courses/by-token", token);
  const c = fromJson(CourseByTokenSchema, json as never, { ignoreUnknownFields: true });
  return { id: c.id, name: c.name, slug: c.slug, year: c.year, term: c.term, optedOut: c.optedOut };
}

export async function optInCourseByToken(token: string): Promise<void> {
  await post<unknown>("/courses/by-token/opt-in", token);
}

export async function optOutCourseByToken(token: string): Promise<void> {
  await post<unknown>("/courses/by-token/opt-out", token);
}

/** The token from the page's query string, "" when the link carried none. */
export function tokenFromQuery(query: Record<string, unknown>): string {
  const token = query.token;
  return typeof token === "string" ? token.trim() : "";
}

export const MISSING_TOKEN_MESSAGE = "This link carries no token. Please open it exactly as it was sent to you.";

export function courseTokenErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 404) return "No course matches this link. Please check that you opened it exactly as it was sent to you.";
    if (err.status === 400) return MISSING_TOKEN_MESSAGE;
  }
  return "Something went wrong. Please try again, or reach out to us.";
}
