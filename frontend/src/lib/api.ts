/**
 * Client for the v2 API.
 *
 * The bearer token is held in memory only, never in localStorage where any XSS could
 * read it, and re-minted from the HttpOnly session cookie by POST /api/v2/auth/token.
 * Losing it on reload is intentional and costs one request at boot.
 */

import {
  fromJson,
  toJson,
  type DescMessage,
  type JsonValue,
  type MessageShape,
} from "@bufbuild/protobuf";

const API_BASE = "/api/v2";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }

  /** True when the caller has no usable session and should be sent to /login. */
  get isUnauthenticated(): boolean {
    return this.status === 401;
  }
}

let accessToken: string | null = null;
/** In-flight token request, shared so concurrent 401s mint only one token. */
let pendingToken: Promise<string> | null = null;

async function requestToken(): Promise<string> {
  const res = await fetch(`${API_BASE}/auth/token`, {
    method: "POST",
    credentials: "same-origin",
    headers: { Accept: "application/json" },
  });

  if (!res.ok) {
    accessToken = null;
    throw new ApiError(res.status, "could not obtain an access token");
  }

  const body = (await res.json()) as { access_token: string };
  accessToken = body.access_token;
  return accessToken;
}

/** Returns a usable token, minting one if necessary. */
function getToken(forceRefresh = false): Promise<string> {
  if (!forceRefresh && accessToken) {
    return Promise.resolve(accessToken);
  }
  if (!pendingToken) {
    pendingToken = requestToken().finally(() => {
      pendingToken = null;
    });
  }
  return pendingToken;
}

/** Drops the cached token, e.g. after logging out. */
export function clearToken(): void {
  accessToken = null;
}

async function toApiError(res: Response): Promise<ApiError> {
  // gRPC-gateway reports failures as {"code":n,"message":"..."}; fall back to the
  // status text when a proxy or non-gateway handler answers instead.
  try {
    const body = await res.json();
    if (body && typeof body.message === "string") {
      return new ApiError(res.status, body.message);
    }
  } catch {
    // response had no JSON body
  }
  return new ApiError(res.status, res.statusText || "request failed");
}

async function send<T>(path: string, init: RequestInit, token: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: "same-origin",
    headers: {
      ...(init.headers ?? {}),
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    },
  });

  if (!res.ok) {
    throw await toApiError(res);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return (await res.json()) as T;
}

/** Authenticated request, retrying once with a fresh token so expiry is invisible. */
export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  let token = await getToken();

  try {
    return await send<T>(path, init, token);
  } catch (err) {
    if (!(err instanceof ApiError) || !err.isUnauthenticated) {
      throw err;
    }
    // Mint a new token and retry once; if the session is gone, requestToken throws
    // and the caller redirects to login.
    token = await getToken(true);
    return await send<T>(path, init, token);
  }
}

/**
 * Request without a bearer token, for endpoints reachable before login. apiFetch would
 * try to mint one from a session that does not exist and fail for the wrong reason.
 */
export async function apiFetchPublic<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: "same-origin",
    headers: { ...(init.headers ?? {}), Accept: "application/json" },
  });

  if (!res.ok) {
    throw await toApiError(res);
  }

  return (await res.json()) as T;
}

export function apiGet<T>(path: string): Promise<T> {
  return apiFetch<T>(path, { method: "GET" });
}

export function apiPatch<T>(path: string, body: unknown): Promise<T> {
  return apiFetch<T>(path, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/*
 * Typed helpers over the generated schemas in src/gen.
 *
 * grpc-gateway marshals with protojson, which @bufbuild/protobuf implements, so the
 * schemas decide how enums, timestamps and 64-bit integers are read. Unknown fields
 * are ignored to match the gateway's DiscardUnknown, so a new server field does not
 * break a deployed client.
 *
 * Paths are written by hand: protoc-gen-es generates message types, not the
 * google.api.http routes.
 */
const JSON_READ_OPTIONS = { ignoreUnknownFields: true } as const;

export async function apiGetMessage<Desc extends DescMessage>(
  schema: Desc,
  path: string,
): Promise<MessageShape<Desc>> {
  const json = await apiGet<JsonValue>(path);
  return fromJson(schema, json, JSON_READ_OPTIONS);
}

/** Typed GET for endpoints that do not require authentication. */
export async function apiGetMessagePublic<Desc extends DescMessage>(
  schema: Desc,
  path: string,
): Promise<MessageShape<Desc>> {
  const json = await apiFetchPublic<JsonValue>(path, { method: "GET" });
  return fromJson(schema, json, JSON_READ_OPTIONS);
}

/** Typed POST for endpoints that do not require authentication. */
export async function apiPostMessagePublic<Req extends DescMessage, Res extends DescMessage>(
  requestSchema: Req,
  responseSchema: Res,
  path: string,
  message: MessageShape<Req>,
): Promise<MessageShape<Res>> {
  const json = await apiFetchPublic<JsonValue>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(toJson(requestSchema, message)),
  });
  return fromJson(responseSchema, json, JSON_READ_OPTIONS);
}

export async function apiPatchMessage<Req extends DescMessage, Res extends DescMessage>(
  requestSchema: Req,
  responseSchema: Res,
  path: string,
  message: MessageShape<Req>,
): Promise<MessageShape<Res>> {
  const json = await apiPatch<JsonValue>(path, toJson(requestSchema, message));
  return fromJson(responseSchema, json, JSON_READ_OPTIONS);
}
