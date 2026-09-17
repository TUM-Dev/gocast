/**
 * API tokens, for the token management page.
 *
 * A token authenticates as the account that created it, so the surface here is
 * small and security-sensitive: `createToken` is the only place its secret is ever
 * readable, `fetchTokens` never returns one, and neither the server nor this client
 * stores it anywhere it could be shown again. Preserve that -- do not add a way to
 * retrieve an existing token's value.
 */

import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";

import {
  CreateTokenRequestSchema,
  ListTokensResponseSchema,
  TokenSecretSchema,
  type Token,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPostMessage } from "./api";

/** model.TokenScope*, in the order the old page offered them. */
export const TOKEN_SCOPES = [
  { value: "lecturer", label: "Scope: lecturer" },
  // Only offered to an administrator by the view; the server refuses it from
  // anyone else regardless of what the client sends.
  { value: "admin", label: "Scope: admin" },
] as const;

export type TokenScope = (typeof TOKEN_SCOPES)[number]["value"];

export interface AdminToken {
  id: number;
  userName: string;
  userEmail: string;
  userLrzId: string;
  scope: string;
  expires: Date | null;
  lastUse: Date | null;
}

/** Who a token authenticates as: the email when there is one, name and LRZ id otherwise. */
export function tokenOwner(token: AdminToken): string {
  return token.userEmail || `${token.userName} ${token.userLrzId}`.trim();
}

function toAdminToken(token: Token): AdminToken {
  return {
    id: token.id,
    userName: token.userName,
    userEmail: token.userEmail,
    userLrzId: token.userLrzId,
    scope: token.scope,
    expires: token.expires ? timestampDate(token.expires) : null,
    lastUse: token.lastUse ? timestampDate(token.lastUse) : null,
  };
}

export interface TokenList {
  tokens: AdminToken[];
  /** Where a lecturer-scoped token points a streaming client, e.g. OBS's "Server" field. */
  rtmpProxyUrl: string;
}

/** Every issued token, without its secret -- see the module comment. */
export async function fetchTokens(): Promise<TokenList> {
  const res = await apiGetMessage(ListTokensResponseSchema, "/admin/tokens");
  return { tokens: res.tokens.map(toAdminToken), rtmpProxyUrl: res.rtmpProxyUrl };
}

/**
 * Creates a token and returns its secret. This is the only response that ever
 * carries it: store it now, because it cannot be asked for again afterwards.
 */
export async function createToken(scope: TokenScope, expires: Date | null): Promise<string> {
  const created = await apiPostMessage(CreateTokenRequestSchema, TokenSecretSchema, "/admin/tokens", {
    $typeName: "protobuf.CreateTokenRequest",
    scope,
    expires: expires ? timestampFromDate(expires) : undefined,
  });
  return created.token;
}

export async function deleteToken(id: number): Promise<void> {
  await apiDelete(`/admin/tokens/${id}`);
}
