/**
 * The deployment's static pages: the three built in (privacy, imprint, about) and any
 * further page an administrator has added. Public content arrives as HTML the server
 * already rendered from Markdown and sanitised; the view inserts it as-is.
 */

import {
  CreateInfoPageRequestSchema,
  GetInfoPageResponseSchema,
  InfoPageSchema,
  ListInfoPagesAdminResponseSchema,
  ListInfoPagesResponseSchema,
  UpdateInfoPageRequestSchema,
  type InfoPage,
} from "@/gen/server/apiv2_pb";
import {
  apiDelete,
  apiGetMessage,
  apiGetMessagePublic,
  apiPatchMessage,
  apiPostMessage,
} from "./api";

export interface AdminInfoPage {
  id: number;
  slug: string;
  name: string;
  rawContent: string;
}

function toAdminInfoPage(page: InfoPage): AdminInfoPage {
  return { id: page.id, slug: page.slug, name: page.name, rawContent: page.rawContent };
}

/**
 * The known slugs, fetched once and cached for the life of the page: a page added by
 * an administrator needs no redeploy to become reachable, but does not change on
 * every navigation either.
 */
let cachedSlugs: Promise<Set<string>> | null = null;

async function fetchInfoPageSlugs(): Promise<Set<string>> {
  if (!cachedSlugs) {
    cachedSlugs = apiGetMessagePublic(ListInfoPagesResponseSchema, "/info-pages")
      .then((res) => new Set(res.pages.map((page) => page.slug)))
      .catch((err) => {
        // A failed fetch should not poison the cache for the next visitor's request.
        cachedSlugs = null;
        throw err;
      });
  }
  return cachedSlugs;
}

/** Whether a path segment is a known info page, without fetching its content. */
export async function isInfoPageSlug(slug: string): Promise<boolean> {
  try {
    return (await fetchInfoPageSlugs()).has(slug);
  } catch {
    return false;
  }
}

/** Fetched without a token: read before signing in, and identical for everyone. */
export async function fetchInfoPage(slug: string): Promise<string> {
  const res = await apiGetMessagePublic(
    GetInfoPageResponseSchema,
    `/info-pages/${encodeURIComponent(slug)}`,
  );
  return res.content;
}

/** Every info page with its raw content, for the administration page. */
export async function fetchInfoPagesAdmin(): Promise<AdminInfoPage[]> {
  const res = await apiGetMessage(ListInfoPagesAdminResponseSchema, "/admin/info-pages");
  return res.pages.map(toAdminInfoPage);
}

/** Creates a page reachable at /{slug} immediately, without a redeploy. */
export async function createInfoPage(
  slug: string,
  name: string,
  rawContent: string,
): Promise<AdminInfoPage> {
  const created = await apiPostMessage(
    CreateInfoPageRequestSchema,
    InfoPageSchema,
    "/admin/info-pages",
    { $typeName: "protobuf.CreateInfoPageRequest", slug, name, rawContent },
  );
  // The slug list was fetched before this page existed; forget it so the next check
  // (e.g. following the link just created) asks the server again.
  cachedSlugs = null;
  return toAdminInfoPage(created);
}

export async function updateInfoPage(
  id: number,
  slug: string,
  name: string,
  rawContent: string,
): Promise<AdminInfoPage> {
  const updated = await apiPatchMessage(
    UpdateInfoPageRequestSchema,
    InfoPageSchema,
    `/admin/info-pages/${id}`,
    { $typeName: "protobuf.UpdateInfoPageRequest", id, slug, name, rawContent },
  );
  cachedSlugs = null;
  return toAdminInfoPage(updated);
}

export async function deleteInfoPage(id: number): Promise<void> {
  await apiDelete(`/admin/info-pages/${id}`);
  cachedSlugs = null;
}
