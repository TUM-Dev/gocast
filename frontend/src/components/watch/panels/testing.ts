/** Test doubles for the panel tests: a clock-and-seek player, and a routed fetch. */

import { vi } from "vitest";

import { clearToken } from "@/lib/api";
import type { PanelPlayer } from "@/lib/watch-panels";

export function fakePlayer(time = 0) {
  const handlers = new Map<string, Set<() => void>>();
  const state = { time };
  const player = {
    getCurrentTime: () => state.time,
    seek: vi.fn((s: number) => {
      state.time = s;
    }),
    on: vi.fn((event: string, fn: () => void) => {
      const set = handlers.get(event) ?? new Set();
      handlers.set(event, set.add(fn));
      return () => set.delete(fn);
    }),
  };
  return {
    player: player as unknown as PanelPlayer & typeof player,
    /** Moves the clock and fires timeupdate, as playback does. */
    tick(seconds: number) {
      state.time = seconds;
      handlers.get("timeupdate")?.forEach((fn) => fn());
    },
    listeners: (event: string) => handlers.get(event)?.size ?? 0,
  };
}

export interface Route {
  method?: string;
  /** Matched against the path and query after /api/v2. */
  path: RegExp | string;
  status?: number;
  body?: unknown;
  /** A text answer, for endpoints that serve a file. */
  text?: string;
}

/** Answers the token request and each route in order of declaration; 404 otherwise. */
export function routeFetch(routes: Route[]) {
  clearToken();
  const calls: { method: string; url: string }[] = [];
  const mock = vi.fn((input: string, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method ?? "GET";
    if (url.endsWith("/auth/token")) {
      return Promise.resolve(new Response(JSON.stringify({ access_token: "t", expires_in: 900 })));
    }
    calls.push({ method, url });
    const rest = url.replace("/api/v2", "");
    const route = routes.find(
      (r) =>
        (r.method ?? "GET") === method &&
        (typeof r.path === "string" ? rest === r.path || rest.startsWith(`${r.path}?`) : r.path.test(rest)),
    );
    if (!route) return Promise.resolve(new Response("{}", { status: 404 }));
    const body = route.text ?? JSON.stringify(route.body ?? {});
    return Promise.resolve(new Response(route.status === 204 ? null : body, { status: route.status ?? 200 }));
  });
  vi.stubGlobal("fetch", mock);
  return { calls, mock };
}
