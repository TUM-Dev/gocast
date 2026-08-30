import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it } from "vitest";
import { createMemoryHistory, createRouter, type Router } from "vue-router";

import AdminLayout from "./AdminLayout.vue";
import type { Permission } from "@/lib/settings";
import { useAuthStore } from "@/stores/auth";

/**
 * The sidebar offers a link exactly when following it works.
 *
 * That is worth testing because the template it replaces got it wrong: it gated the
 * whole Administration block on `Role == 1`, which stopped matching the server once
 * web/router.go split those routes across PermAdministerServer and PermManageUsers.
 * The nav is not a security boundary — the server refuses either way — but a link
 * that 403s is a bug, and so is a missing link to a page someone may use.
 */

const blank = { template: "<div />" };

function makeRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/admin/runners", name: "admin-runners", component: blank }],
  });
}

function mountAs(permissions: Permission[]) {
  const auth = useAuthStore();
  auth.user = { id: 1, name: "Test", email: "t@example.com", role: 1, permissions } as never;

  return mount(AdminLayout, { global: { plugins: [makeRouter()] } });
}

/** The labels of every link the sidebar is currently offering. */
function links(wrapper: ReturnType<typeof mountAs>): string[] {
  return wrapper.findAll("nav a").map((link) => link.text());
}

beforeEach(() => {
  setActivePinia(createPinia());
});

describe("the administration sidebar", () => {
  it("offers a server administrator the pages they administer", async () => {
    const wrapper = mountAs(["server.administer"]);
    await wrapper.vm.$nextTick();

    expect(links(wrapper)).toContain("Runners");
    expect(links(wrapper)).toContain("Maintenance");
  });

  it("withholds the user pages from someone who only administers the server", async () => {
    // web/router.go puts /admin/users and /admin/token behind PermManageUsers, not
    // PermAdministerServer. Both belong to admins today, so the two only come apart
    // when an operator role is added — which is the case this guards.
    const wrapper = mountAs(["server.administer"]);
    await wrapper.vm.$nextTick();

    expect(links(wrapper)).not.toContain("Users");
    expect(links(wrapper)).not.toContain("Token Management");
  });

  it("offers only the user pages to someone who only manages users", async () => {
    const wrapper = mountAs(["users.manage"]);
    await wrapper.vm.$nextTick();

    expect(links(wrapper)).toEqual(["Users", "Token Management"]);
  });

  it("offers a lecturer their courses and no administration at all", async () => {
    const wrapper = mountAs(["lecture"]);
    await wrapper.vm.$nextTick();

    expect(links(wrapper)).toEqual(["Schedule", "Create Course"]);
  });

  it("offers a student nothing", async () => {
    // They cannot reach an admin page to see this, but a client-side navigation from
    // one they can reach must not render a menu of links that all refuse them.
    const wrapper = mountAs([]);
    await wrapper.vm.$nextTick();

    expect(links(wrapper)).toEqual([]);
  });

  it("routes the migrated page in the client and leaves the rest to the server", async () => {
    // A RouterLink to a path the SPA does not own would match nothing and bounce
    // through the server anyway; a plain href says what is actually happening.
    const wrapper = mountAs(["server.administer"]);
    await wrapper.vm.$nextTick();

    const runners = wrapper.findAll("nav a").find((link) => link.text() === "Runners");
    expect(runners?.attributes("href")).toBe("/admin/runners");
    expect(runners?.element.className).not.toContain("text-5");
  });
});
