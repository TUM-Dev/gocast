import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter, type Router } from "vue-router";

import AdminLayout from "./AdminLayout.vue";
import { fetchAdministeredCourses, type AdministeredCourse } from "@/lib/course-admin";
import { fetchSemesters } from "@/lib/semesters";
import type { Permission } from "@/lib/settings";
import { useAuthStore } from "@/stores/auth";

vi.mock("@/lib/course-admin", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/course-admin")>()),
  fetchAdministeredCourses: vi.fn(),
}));
vi.mock("@/lib/semesters", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/semesters")>()),
  fetchSemesters: vi.fn(),
}));

const prof1Courses: AdministeredCourse[] = [
  { id: 1, name: "Einführung Brauereiwesen", slug: "brauereiwesen", year: 2022, term: "S" },
  { id: 4, name: "Fortgeschrittene Bierkunde", slug: "bierkunde", year: 2022, term: "S" },
  { id: 3, name: "Praktikum: Golang", slug: "godev", year: 2021, term: "W" },
];

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
      { path: "/admin/courses/:courseID/settings", component: blank },
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
  vi.mocked(fetchAdministeredCourses).mockReset().mockResolvedValue([]);
  vi.mocked(fetchSemesters)
    .mockReset()
    .mockResolvedValue({ current: { year: 2022, term: "S" }, all: [] });
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

  describe("the tree of administered courses", () => {
    it("groups a lecturer's courses by semester, newest first, linking to their settings", async () => {
      vi.mocked(fetchAdministeredCourses).mockResolvedValue(prof1Courses);
      const wrapper = mountAs(["lecture"]);
      await flushPromises();

      const groups = wrapper.findAll("nav details");
      expect(groups.map((g) => g.find("summary").text())).toEqual(["Summer 2022", "Winter 2021/22"]);
      expect(groups[0].findAll("a").map((a) => a.text())).toEqual([
        "Einführung Brauereiwesen",
        "Fortgeschrittene Bierkunde",
      ]);
      expect(groups[1].find("a").attributes("href")).toBe("/admin/courses/3/settings");
    });

    it("expands only the current semester", async () => {
      vi.mocked(fetchAdministeredCourses).mockResolvedValue(prof1Courses);
      const wrapper = mountAs(["lecture"]);
      await flushPromises();

      const open = wrapper.findAll("nav details").map((g) => (g.element as HTMLDetailsElement).open);
      expect(open).toEqual([true, false]);
    });

    it("is not asked for without the lecture permission", async () => {
      // listAdministeredCourses refuses them; asking would only log a 403.
      const wrapper = mountAs(["users.manage"]);
      await flushPromises();

      expect(fetchAdministeredCourses).not.toHaveBeenCalled();
      expect(wrapper.findAll("nav details")).toHaveLength(0);
    });

    it("leaves the sidebar's links alone when the tree cannot be loaded", async () => {
      vi.mocked(fetchAdministeredCourses).mockRejectedValue(new Error("down"));
      const wrapper = mountAs(["lecture"]);
      await flushPromises();

      expect(links(wrapper)).toEqual(["Schedule", "Create Course"]);
    });
  });
});
