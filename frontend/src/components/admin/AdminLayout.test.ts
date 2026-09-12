import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it } from "vitest";
import { createMemoryHistory, createRouter, type Router } from "vue-router";

import AdminLayout from "./AdminLayout.vue";
import type { Permission } from "@/lib/settings";
import { useAuthStore } from "@/stores/auth";

/**
 * The sidebar offers a link exactly when following it works. The template it replaces
 * gated the whole block on `Role == 1`, which stopped matching the server once those
 * routes split across two permissions.
 */

const blank = { template: "<div />" };

function makeRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/admin/runners", name: "admin-runners", component: blank },
      { path: "/admin/users", name: "admin-users", component: blank },
    ],
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
    // Both belong to admins today, so they only come apart with an operator role.
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

  it("routes both migrated pages in the client", async () => {
    const wrapper = mountAs(["server.administer", "users.manage"]);
    await wrapper.vm.$nextTick();

    const migrated = wrapper
      .findAll("nav a")
      .filter((link) => ["Runners", "Users"].includes(link.text()));
    expect(migrated).toHaveLength(2);
    for (const link of migrated) {
      // Both render the same href, so the class is what tells them apart.
      expect(link.element.className).not.toContain("text-5");
    }
  });

  it("offers a lecturer their courses and no administration at all", async () => {
    const wrapper = mountAs(["lecture"]);
    await wrapper.vm.$nextTick();

    expect(links(wrapper)).toEqual(["Schedule", "Create Course"]);
  });

  it("offers a student nothing", async () => {
    // A client-side navigation must not render a menu of links that all refuse them.
    const wrapper = mountAs([]);
    await wrapper.vm.$nextTick();

    expect(links(wrapper)).toEqual([]);
  });

  it("routes the migrated page in the client and leaves the rest to the server", async () => {
    // A plain href says what is actually happening.
    const wrapper = mountAs(["server.administer"]);
    await wrapper.vm.$nextTick();

    const runners = wrapper.findAll("nav a").find((link) => link.text() === "Runners");
    expect(runners?.attributes("href")).toBe("/admin/runners");
    expect(runners?.element.className).not.toContain("text-5");
  });
});
