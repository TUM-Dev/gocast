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

/** How long a failed token request is remembered when the server names no time. */
const TOKEN_FAILURE_COOLDOWN_MS = 5_000;
const TOKEN_FAILURE_COOLDOWN_MAX_MS = 60_000;

/**
 * The last token request that failed for a reason other than a missing session (429,
 * 5xx), and until when to answer with it instead of asking again. Without it every API
 * call of a client the server is shedding mints another token, which is the load that
 * made the server shed it.
 */
let tokenFailure: { error: ApiError; until: number } | null = null;

function cooldownOf(res: Response): number {
  const seconds = Number(res.headers.get("Retry-After"));
  if (Number.isFinite(seconds) && seconds > 0) {
    return Math.min(seconds * 1000, TOKEN_FAILURE_COOLDOWN_MAX_MS);
  }
  return TOKEN_FAILURE_COOLDOWN_MS;
}

async function requestToken(): Promise<string> {
  const res = await fetch(`${API_BASE}/auth/token`, {
    method: "POST",
    credentials: "same-origin",
    headers: { Accept: "application/json" },
  });

  if (!res.ok) {
    accessToken = null;
    const error = new ApiError(res.status, "could not obtain an access token");
    // A 401 is answered once per page by sessionAbsent; anything else is worth backing off from.
    if (!error.isUnauthenticated) {
      tokenFailure = { error, until: Date.now() + cooldownOf(res) };
    }
    throw error;
  }

  const body = (await res.json()) as { access_token: string };
  accessToken = body.access_token;
  tokenFailure = null;
  return accessToken;
}

/** Returns a usable token, minting one if necessary. */
function getToken(forceRefresh = false): Promise<string> {
  if (!forceRefresh && accessToken) {
    return Promise.resolve(accessToken);
  }
  if (tokenFailure && Date.now() < tokenFailure.until) {
    return Promise.reject(tokenFailure.error);
  }
  if (!pendingToken) {
    pendingToken = requestToken().finally(() => {
      pendingToken = null;
    });
  }
  return pendingToken;
}

/**
 * Remembers that there is no session to mint a token from, so the endpoints that
 * serve anonymous callers stop asking for one on every request. Only ever set from a
 * 401: a token request that failed for any other reason has not answered the question.
 */
let sessionAbsent = false;

/** Drops the cached token, e.g. after logging out. */
export function clearToken(): void {
  accessToken = null;
  sessionAbsent = false;
  tokenFailure = null;
}

/** A token, or null when nobody is signed in. Anything else still throws. */
async function tokenOrNull(forceRefresh = false): Promise<string | null> {
  if (sessionAbsent) {
    return null;
  }

  try {
    return await getToken(forceRefresh);
  } catch (err) {
    if (err instanceof ApiError && err.isUnauthenticated) {
      sessionAbsent = true;
      return null;
    }
    throw err;
  }
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

/**
 * Whether the browser has a session to authenticate with.
 *
 * Answers from the token endpoint rather than from the user's profile, which is a
 * round trip further on: minting a token succeeds exactly when there is a session, so
 * anything that only needs to know whether to call an authenticated endpoint can ask
 * here and start a round trip earlier.
 */
export async function hasSession(): Promise<boolean> {
  return (await tokenOrNull()) !== null;
}

/**
 * A freshly minted token, or null when nobody is signed in — for the realtime socket,
 * which presents its token once per connection rather than per request, so a cached
 * one could be about to expire by the time it reconnects.
 */
export function freshTokenOrNull(): Promise<string | null> {
  return tokenOrNull(true);
}

/**
 * Request for an endpoint that answers anonymous callers but shows a signed-in one
 * more — the course listings, which gain the logged-in-only courses, and the
 * thumbnails and streams behind them.
 *
 * Neither apiFetch nor apiFetchPublic fits: the first fails for a visitor who is
 * simply not signed in, and the second would quietly show a signed-in user the
 * anonymous view of the site.
 *
 * Signing in is a full page load — the form posts to Go — so a session that appears
 * later in the life of this module is not a case that arises.
 */
export async function apiFetchOptionalAuth<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = await tokenOrNull();
  if (token === null) {
    return apiFetchPublic<T>(path, init);
  }
  return apiFetch<T>(path, init);
}

export function apiGet<T>(path: string): Promise<T> {
  return apiFetch<T>(path, { method: "GET" });
}

export function apiPost<T>(path: string, body: unknown): Promise<T> {
  return apiFetch<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/**
 * POST of a multipart form, for the upload endpoints beside the gateway. No
 * Content-Type of our own: the browser sets it with the boundary the body needs. Their
 * failures come in the gateway's shape, so they surface as the same ApiError.
 *
 * fetch reports no upload progress, so a caller that passes onProgress is sent through
 * XMLHttpRequest instead; the rest keep the plain fetch path.
 */
export function apiUpload<T>(path: string, form: FormData, onProgress?: (fraction: number) => void): Promise<T> {
  if (!onProgress) {
    return apiFetch<T>(path, { method: "POST", body: form });
  }
  return apiUploadWithProgress<T>(path, form, onProgress);
}

/**
 * The XHR twin of apiFetch, with upload progress.
 *
 * It mints a fresh token instead of retrying on a 401. The server checks the token
 * before it reads the body, so one that is valid when the request starts carries an
 * upload of any length. A cached token that had expired would be refused at once, and
 * with a large body still going out the browser may report a reset connection rather
 * than the 401, so the retry apiFetch relies on would not be reached.
 */
async function apiUploadWithProgress<T>(
  path: string,
  form: FormData,
  onProgress: (fraction: number) => void,
): Promise<T> {
  const token = await getToken(true);

  return new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `${API_BASE}${path}`);
    xhr.withCredentials = true;
    xhr.setRequestHeader("Accept", "application/json");
    xhr.setRequestHeader("Authorization", `Bearer ${token}`);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && e.total > 0) onProgress(e.loaded / e.total);
    };
    // fetch rejects with a TypeError when the request does not complete, and callers
    // already treat that as "something went wrong"; match it, so none of them has to
    // know a second kind of failure.
    xhr.onerror = () => reject(new TypeError("network error"));
    xhr.onabort = () => reject(new TypeError("upload aborted"));
    xhr.ontimeout = () => reject(new TypeError("upload timed out"));
    xhr.onload = async () => {
      if (xhr.status < 200 || xhr.status >= 300) {
        reject(await toApiError(new Response(xhr.responseText, { status: xhr.status, statusText: xhr.statusText })));
        return;
      }
      if (xhr.status === 204 || !xhr.responseText) {
        resolve(undefined as T);
        return;
      }
      try {
        resolve(JSON.parse(xhr.responseText) as T);
      } catch {
        reject(new ApiError(xhr.status, "invalid response"));
      }
    };
    xhr.send(form);
  });
}

/** DELETE, for endpoints that answer with an empty body. */
export function apiDelete(path: string): Promise<void> {
  return apiFetch<void>(path, { method: "DELETE" });
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

/** Typed GET for endpoints that serve anonymous callers but show more to a user. */
export async function apiGetMessageOptionalAuth<Desc extends DescMessage>(
  schema: Desc,
  path: string,
): Promise<MessageShape<Desc>> {
  const json = await apiFetchOptionalAuth<JsonValue>(path, { method: "GET" });
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

/** Typed POST for endpoints that require authentication. */
export async function apiPostMessage<Req extends DescMessage, Res extends DescMessage>(
  requestSchema: Req,
  responseSchema: Res,
  path: string,
  message: MessageShape<Req>,
): Promise<MessageShape<Res>> {
  const json = await apiPost<JsonValue>(path, toJson(requestSchema, message));
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

export function apiPut<T>(path: string, body: unknown): Promise<T> {
  return apiFetch<T>(path, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** Typed PUT, for the endpoints that replace a whole collection at once. */
export async function apiPutMessage<Req extends DescMessage, Res extends DescMessage>(
  requestSchema: Req,
  responseSchema: Res,
  path: string,
  message: MessageShape<Req>,
): Promise<MessageShape<Res>> {
  const json = await apiPut<JsonValue>(path, toJson(requestSchema, message));
  return fromJson(responseSchema, json, JSON_READ_OPTIONS);
}
