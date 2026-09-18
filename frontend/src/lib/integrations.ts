import {
  CreateIntegrationRequestSchema,
  CreateIntegrationResponseSchema,
  ListIntegrationsResponseSchema,
  RotateIntegrationKeyRequestSchema,
  RotateIntegrationKeyResponseSchema,
  type IntegrationSummary,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPostMessage } from "./api";

export type Integration = Pick<IntegrationSummary, "id" | "name" | "returnUrl" | "hasKey">;

export async function fetchIntegrations(): Promise<Integration[]> {
  const response = await apiGetMessage(ListIntegrationsResponseSchema, "/admin/integrations");
  return response.integrations;
}

export function createIntegration(name: string, returnUrl: string) {
  return apiPostMessage(
    CreateIntegrationRequestSchema,
    CreateIntegrationResponseSchema,
    "/admin/integrations",
    { $typeName: "protobuf.CreateIntegrationRequest", name, returnUrl },
  );
}

export async function rotateIntegrationKey(id: number): Promise<string> {
  const response = await apiPostMessage(
    RotateIntegrationKeyRequestSchema,
    RotateIntegrationKeyResponseSchema,
    `/admin/integrations/${id}/key`,
    { $typeName: "protobuf.RotateIntegrationKeyRequest", id },
  );
  return response.apiKey;
}

export async function revokeIntegrationKey(id: number): Promise<void> {
  await apiDelete(`/admin/integrations/${id}/key`);
}
