/**
 * The consent page an application sends a lecturer to (`/integration/authorize/:id`)
 * before it may read a course. Approving hands the application a one-time code, so
 * both calls go out with the bearer token (apiFetch always attaches one): the server
 * refuses a cookie-only request, which a foreign page could make the browser send.
 */

import {
  DecideIntegrationAuthorizationRequestSchema,
  DecideIntegrationAuthorizationResponseSchema,
  IntegrationAuthorizationDecision,
  IntegrationAuthorizationSchema,
  type IntegrationAuthorization,
} from "@/gen/server/apiv2_pb";
import { ApiError, apiGetMessage, apiPostMessage } from "./api";

export type { IntegrationAuthorization };

const path = (id: number) => `/integration-authorizations/${id}`;

export function fetchIntegrationAuthorization(id: number, state: string): Promise<IntegrationAuthorization> {
  return apiGetMessage(IntegrationAuthorizationSchema, `${path(id)}?state=${encodeURIComponent(state)}`);
}

/**
 * Records the answer and returns where to send the browser: the application's
 * registered return URL with the state and either a code or `error=access_denied`.
 */
export async function decideIntegrationAuthorization(
  id: number,
  state: string,
  decision: "approve" | "cancel",
  courseId = 0,
): Promise<string> {
  const response = await apiPostMessage(
    DecideIntegrationAuthorizationRequestSchema,
    DecideIntegrationAuthorizationResponseSchema,
    `${path(id)}/decision`,
    {
      $typeName: "protobuf.DecideIntegrationAuthorizationRequest",
      integrationId: id,
      state,
      decision:
        decision === "approve"
          ? IntegrationAuthorizationDecision.APPROVE
          : IntegrationAuthorizationDecision.CANCEL,
      courseId,
    },
  );
  return response.redirectUrl;
}

export function integrationAuthorizationErrorMessage(err: unknown): string {
  // The server's wording is written for this page, so it is shown as it is.
  if (err instanceof ApiError && [400, 403, 404].includes(err.status) && err.message) {
    return err.message.charAt(0).toUpperCase() + err.message.slice(1) + ".";
  }
  return "Something went wrong. Reload this page to try again.";
}
