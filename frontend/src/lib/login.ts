/**
 * Login page data.
 *
 * Credentials themselves are never sent from here: the form posts to /login, which is
 * still handled by Go so that it can set the session cookie and honour the stored
 * redirect. This module only covers what the page needs to render, plus the password
 * reset request.
 */

import { create } from "@bufbuild/protobuf";

import {
  GetLoginOptionsResponseSchema,
  ResetPasswordRequestSchema,
  ResetPasswordResponseSchema,
} from "@/gen/server/apiv2_pb";
import { apiGetMessagePublic, apiPostMessagePublic } from "./api";

export interface LoginOptions {
  useSaml: boolean;
  idpName: string;
  idpColor: string;
}

/** Colour used for the identity provider button when the config sets none. */
const DEFAULT_IDP_COLOR = "#3070B3";

export async function fetchLoginOptions(): Promise<LoginOptions> {
  const res = await apiGetMessagePublic(GetLoginOptionsResponseSchema, "/login-options");

  return {
    useSaml: res.useSaml,
    idpName: res.idpName,
    idpColor: res.idpColor === "" ? DEFAULT_IDP_COLOR : res.idpColor,
  };
}

/**
 * Requests a password reset mail.
 *
 * The response is deliberately the same whether or not the address is known, so the
 * caller cannot use this to discover which accounts exist.
 */
export async function requestPasswordReset(email: string): Promise<void> {
  await apiPostMessagePublic(
    ResetPasswordRequestSchema,
    ResetPasswordResponseSchema,
    "/users/reset-password",
    create(ResetPasswordRequestSchema, { email }),
  );
}
