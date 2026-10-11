/**
 * The set-password page, reached from the link a password reset or an account
 * invite mails. The key in that link is the only credential; both calls are public.
 */

import { ApiError, apiFetchPublic } from "./api";

const path = (key: string) => `/users/password-reset/${encodeURIComponent(key)}`;

/** True while the key still opens the page; false once it was used or never issued. */
export async function checkPasswordResetKey(key: string): Promise<boolean> {
  try {
    await apiFetchPublic<unknown>(path(key));
    return true;
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return false;
    throw err;
  }
}

export async function setPasswordByResetKey(key: string, password: string): Promise<void> {
  await apiFetchPublic<unknown>(path(key), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password }),
  });
}

export const MIN_PASSWORD_LENGTH = 8;

/** What keeps the form from being sent, or "" when nothing does. */
export function passwordProblem(password: string, confirmation: string): string {
  if (password.length < MIN_PASSWORD_LENGTH) return `The password must be at least ${MIN_PASSWORD_LENGTH} characters long.`;
  if (password !== confirmation) return "The passwords do not match.";
  return "";
}

export function setPasswordErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 404) return "This link is not valid any more. Request a new one from the login page.";
    if (err.status === 400) return err.message;
  }
  return "Something went wrong. Please try again.";
}
