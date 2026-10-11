/**
 * The fresh-installation page: the one account that can be created before anyone
 * can sign in. The server allows it only while the users table is empty.
 */

import { ApiError, apiFetchPublic } from "./api";

export interface FirstUser {
  name: string;
  email: string;
  password: string;
}

export async function createFirstUser(user: FirstUser): Promise<void> {
  await apiFetchPublic<unknown>("/users/first", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(user),
  });
}

export const MIN_PASSWORD_LENGTH = 8;

/** What is wrong with each field, by field; empty when the form can be sent. */
export function firstUserProblems(user: FirstUser): Partial<Record<keyof FirstUser, string>> {
  const problems: Partial<Record<keyof FirstUser, string>> = {};
  // The old page wanted a first and a last name.
  if (user.name.trim().length < 5 || !/\S\s+\S/.test(user.name.trim())) {
    problems.name = "Please enter your first and last name.";
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(user.email.trim())) {
    problems.email = "Please enter a valid email address.";
  }
  if (user.password.length < MIN_PASSWORD_LENGTH) {
    problems.password = `The password must be at least ${MIN_PASSWORD_LENGTH} characters long.`;
  }
  return problems;
}

export function firstUserErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 403) return "This deployment already has users. Sign in as an administrator to create more.";
    if (err.status === 400) return err.message;
  }
  return "Something went wrong. Please try again.";
}
