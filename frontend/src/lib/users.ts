/**
 * Account administration, for the users page.
 *
 * The staff list shows contact details as they are; search results arrive masked.
 * That difference is inherited from the page this replaces — see ListStaff in
 * apiv2/server/user_admin.go.
 */

import {
  CreateUserRequestSchema,
  ListUsersResponseSchema,
  UpdateUserRoleRequestSchema,
  UserSummarySchema,
  type UserSummary,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPatchMessage, apiPostMessage } from "./api";

/** model.User.Role, named once rather than spelled out at each comparison. */
export const Role = {
  admin: 1,
  lecturer: 2,
  /** Someone invited to a single course, who has no account of their own. */
  generic: 3,
  student: 4,
} as const;

export type RoleValue = (typeof Role)[keyof typeof Role];

/** What the role selector offers, in the order the old page listed them. */
export const ASSIGNABLE_ROLES: { value: RoleValue; label: string }[] = [
  { value: Role.admin, label: "Admin" },
  { value: Role.lecturer, label: "Lecturer" },
  { value: Role.student, label: "Student" },
];

export function roleLabel(role: number): string {
  switch (role) {
    case Role.admin:
      return "Admin";
    case Role.lecturer:
      return "Lecturer";
    case Role.generic:
      return "Invited";
    case Role.student:
      return "Student";
    default:
      // Still a role the server enforces, so show it rather than guess.
      return `Role ${role}`;
  }
}

export interface AdminUser {
  id: number;
  name: string;
  /** Masked in search results; empty when the address could not be masked. */
  email: string;
  /** Masked in search results. */
  lrzId: string;
  role: number;
}

/** The minimum a search box has to hold before it asks the server, mirroring it. */
export const SEARCH_MIN_LENGTH = 3;

function toAdminUser(user: UserSummary): AdminUser {
  return { id: user.id, name: user.name, email: user.email, lrzId: user.lrzId, role: user.role };
}

/** The administrators and lecturers, with contact details unmasked. */
export async function fetchStaff(): Promise<AdminUser[]> {
  const res = await apiGetMessage(ListUsersResponseSchema, "/admin/users");
  return res.users.map(toAdminUser);
}

/** Searches every account, with contact details masked. */
export async function searchUsers(query: string, role?: RoleValue): Promise<AdminUser[]> {
  const params = new URLSearchParams();
  if (query) params.set("query", query);
  if (role !== undefined) params.set("role", String(role));

  const res = await apiGetMessage(ListUsersResponseSchema, `/admin/users/search?${params}`);
  return res.users.map(toAdminUser);
}

/** Whether a search with these inputs would be accepted, so the page need not send it. */
export function searchable(query: string, role?: RoleValue): boolean {
  return role !== undefined || query.trim().length >= SEARCH_MIN_LENGTH;
}

/** Creates a lecturer account and has an invitation emailed to it. */
export async function createUser(name: string, email: string): Promise<AdminUser> {
  const created = await apiPostMessage(CreateUserRequestSchema, UserSummarySchema, "/admin/users", {
    $typeName: "protobuf.CreateUserRequest",
    name,
    email,
  });
  return toAdminUser(created);
}

export async function updateUserRole(id: number, role: RoleValue): Promise<AdminUser> {
  const updated = await apiPatchMessage(
    UpdateUserRoleRequestSchema,
    UserSummarySchema,
    `/admin/users/${id}/role`,
    { $typeName: "protobuf.UpdateUserRoleRequest", userId: id, role },
  );
  return toAdminUser(updated);
}

export async function deleteUser(id: number): Promise<void> {
  await apiDelete(`/admin/users/${id}`);
}

/**
 * Signs the caller in as another account, replacing their own session. Still v1: it
 * creates a session, which the SPA does not manage. Leave by full navigation after.
 */
export async function impersonate(id: number): Promise<void> {
  const response = await fetch("/api/users/impersonate", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id }),
  });

  if (!response.ok) {
    throw new Error("could not impersonate that account");
  }
}
