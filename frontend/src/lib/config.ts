/**
 * What the deployment says about itself: branding, version and links.
 *
 * Public and identical for everyone, so it is fetched once and shared. The templates
 * get this by interpolating IndexData; the SPA has to ask.
 */

import { GetFrontendConfigResponseSchema } from "@/gen/server/apiv2_pb";
import { apiGetMessagePublic } from "./api";

export interface FrontendConfig {
  title: string;
  description: string;
  /** Build identifier, appended to the footer's project name as `gocast@…`. */
  versionTag: string;
  /** Optional; the footer links to it only when the deployment sets one. */
  wikiUrl: string;
  /** True while the deployment has no users, when the first account is still to be made. */
  isFreshInstallation: boolean;
}

/**
 * Shared across callers and kept for the life of the page.
 *
 * Nothing here changes without a deployment, and the footer alone mounts twice on
 * the start page -- once for the desktop layout and once inside the sidebar for the
 * mobile one -- so without this the page asks the same question twice on every load.
 * A failure is not cached, so the next caller retries.
 */
let pending: Promise<FrontendConfig> | null = null;

export function fetchConfig(): Promise<FrontendConfig> {
  pending ??= request().catch((err: unknown) => {
    pending = null;
    throw err;
  });
  return pending;
}

/** Drops the cached configuration. For tests. */
export function resetConfig(): void {
  pending = null;
}

async function request(): Promise<FrontendConfig> {
  const res = await apiGetMessagePublic(GetFrontendConfigResponseSchema, "/config");

  return {
    title: res.branding?.title ?? "",
    description: res.branding?.description ?? "",
    versionTag: res.versionTag,
    wikiUrl: res.wikiUrl,
    isFreshInstallation: res.isFreshInstallation,
  };
}
