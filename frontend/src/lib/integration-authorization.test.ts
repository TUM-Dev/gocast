import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { ApiError, clearToken } from "./api";
import {
  decideIntegrationAuthorization,
  fetchIntegrationAuthorization,
  integrationAuthorizationErrorMessage,
} from "./integration-authorization";

let fetchMock: ReturnType<typeof vi.fn>;

function respondWith(body: unknown, status = 200): void {
  fetchMock.mockImplementation((url: string) =>
    Promise.resolve(
      new Response(JSON.stringify(url.endsWith("/auth/token") ? { access_token: "t" } : body), {
        status: url.endsWith("/auth/token") ? 200 : status,
      }),
    ),
  );
}

const calls = () => fetchMock.mock.calls.filter(([url]) => !String(url).endsWith("/auth/token"));

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

it("reads the request to authorize with the state in the query", async () => {
  respondWith({
    integrationName: "Portal",
    userName: "Prof One",
    courses: [{ id: 11, name: "Example", teachingTerm: "W", year: 2026 }],
  });

  const authorization = await fetchIntegrationAuthorization(7, "a+b/c");

  expect(authorization.courses[0].name).toBe("Example");
  expect(calls()[0][0]).toBe("/api/v2/integration-authorizations/7?state=a%2Bb%2Fc");
});

it("sends the decision with a bearer token and returns the redirect URL", async () => {
  respondWith({ redirectUrl: "https://portal.example/cb?code=x&state=s" });

  const target = await decideIntegrationAuthorization(7, "s", "approve", 11);

  expect(target).toBe("https://portal.example/cb?code=x&state=s");
  const [url, init] = calls()[0];
  expect(url).toBe("/api/v2/integration-authorizations/7/decision");
  expect(init.headers.Authorization).toBe("Bearer t");
  expect(JSON.parse(init.body)).toMatchObject({
    integrationId: 7,
    state: "s",
    decision: "INTEGRATION_AUTHORIZATION_DECISION_APPROVE",
    courseId: 11,
  });
});

it("sends a cancellation as such", async () => {
  respondWith({ redirectUrl: "https://portal.example/cb?error=access_denied" });
  await decideIntegrationAuthorization(7, "s", "cancel");
  expect(JSON.parse(calls()[0][1].body).decision).toBe("INTEGRATION_AUTHORIZATION_DECISION_CANCEL");
});

it("shows the server's explanation for a refusal and a generic one otherwise", () => {
  expect(integrationAuthorizationErrorMessage(new ApiError(404, "that course is gone"))).toBe(
    "That course is gone.",
  );
  expect(integrationAuthorizationErrorMessage(new ApiError(500, "boom"))).toMatch(/Something went wrong/);
});
