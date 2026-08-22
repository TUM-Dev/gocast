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

export async function fetchConfig(): Promise<FrontendConfig> {
  const res = await apiGetMessagePublic(GetFrontendConfigResponseSchema, "/config");

  return {
    title: res.branding?.title ?? "",
    description: res.branding?.description ?? "",
    versionTag: res.versionTag,
    wikiUrl: res.wikiUrl,
    isFreshInstallation: res.isFreshInstallation,
  };
}
