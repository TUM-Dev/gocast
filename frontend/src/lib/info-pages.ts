/**
 * The deployment's static pages. The content arrives as HTML the server already
 * rendered from Markdown and sanitised; the view inserts it as-is.
 */

import { GetInfoPageResponseSchema } from "@/gen/server/apiv2_pb";
import { apiGetMessagePublic } from "./api";

/** The pages the server answers for. The name is the route, not the editable title. */
export const INFO_PAGE_NAMES = ["privacy", "imprint", "about"] as const;

export type InfoPageName = (typeof INFO_PAGE_NAMES)[number];

export function isInfoPageName(value: string): value is InfoPageName {
  return (INFO_PAGE_NAMES as readonly string[]).includes(value);
}

/** Fetched without a token: read before signing in, and identical for everyone. */
export async function fetchInfoPage(name: InfoPageName): Promise<string> {
  const res = await apiGetMessagePublic(GetInfoPageResponseSchema, `/info-pages/${name}`);
  return res.content;
}
