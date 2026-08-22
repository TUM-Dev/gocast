import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter, type Router } from "vue-router";

import StartPageSidenav from "./StartPageSidenav.vue";
import { useCourseStore } from "@/stores/courses";
import { useSemesterStore } from "@/stores/semester";
import { useSidenavStore } from "@/stores/sidenav";

/**
 * The sidebar is the start page's navigation, and the parts worth covering are the
 * ones that were tuning decisions on the old page rather than anything a type checks:
 * how many courses each group shows before handing off to a full listing, and which
 * groups appear at all.
 */

const { fetchSemesters } = vi.hoisted(() => ({ fetchSemesters: vi.fn() }));
vi.mock("@/lib/semesters", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/semesters")>()),
  fetchSemesters,
}));

const blank = { template: "<div />" };

function makeRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/", name: "home", component: blank },
      { path: "/courses/mine", name: "my-courses", component: blank },
      { path: "/courses/public", name: "public-courses", component: blank },
      { path: "/course/:year/:term/:slug", name: "course", component: blank },
    ],
  });
}

const courses = (count: number, prefix: string) =>
  Array.from({ length: count }, (_, i) => ({
    id: i + 1,
    name: `${prefix} ${i + 1}`,
    slug: `${prefix}-${i + 1}`.toLowerCase(),
    semester: { year: 2026, term: "W" },
    pinned: false,
  })) as never[];

let router: Router;

async function render(listings: Partial<Record<"user" | "public" | "pinned", unknown[]>> = {}) {
  const store = useCourseStore();
  store.userCourses = (listings.user ?? []) as never[];
  store.publicCourses = (listings.public ?? []) as never[];
  store.pinnedCourses = (listings.pinned ?? []) as never[];

  const wrapper = mount(StartPageSidenav, { global: { plugins: [router] } });
  await flushPromises();
  return wrapper;
}

beforeEach(async () => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  fetchSemesters.mockResolvedValue({
    current: { year: 2026, term: "W" },
    all: [
      { year: 2026, term: "W" },
      { year: 2026, term: "S" },
    ],
  });
  router = makeRouter();
  router.push("/");
  await router.isReady();
});

describe("course groups", () => {
  it("shows at most eight of the user's own courses, then offers the full listing", async () => {
    const wrapper = await render({ user: courses(10, "Mine") });

    const links = wrapper.findAll('a[href^="/course/"]');
    expect(links).toHaveLength(8);
    expect(wrapper.text()).toContain("Show all my courses");
  });

  it("offers no full listing when everything already fits", async () => {
    const wrapper = await render({ user: courses(3, "Mine") });

    expect(wrapper.text()).not.toContain("Show all my courses");
  });

  it("shows at most five public courses, then offers the full listing", async () => {
    const wrapper = await render({ public: courses(9, "Public") });

    expect(wrapper.findAll('a[href^="/course/"]')).toHaveLength(5);
    expect(wrapper.text()).toContain("Show all public courses");
  });

  it("leaves out the groups that have nothing in them", async () => {
    const wrapper = await render();

    expect(wrapper.text()).not.toContain("Pinned Courses");
    expect(wrapper.text()).not.toContain("My Courses");
    // Public courses keep their header either way, as on the old page.
    expect(wrapper.text()).toContain("Public Courses");
  });

  it("shows the pinned group when there is something pinned", async () => {
    const wrapper = await render({ pinned: courses(2, "Pinned") });

    expect(wrapper.text()).toContain("Pinned Courses");
  });
});

describe("the semester picker", () => {
  it("shows only the selected semester until asked for all of them", async () => {
    const wrapper = await render();

    expect(wrapper.text()).toContain("Winter 2026/27");
    expect(wrapper.text()).not.toContain("Summer 2026");

    await wrapper.get("button").trigger("click");
    expect(wrapper.text()).toContain("Summer 2026");
  });

  it("switches semester through the URL rather than internal state", async () => {
    // The semester lives in the query so a link carries it and the back button works.
    const wrapper = await render();
    await wrapper.get("button").trigger("click");

    const summer = wrapper.findAll("button").find((b) => b.text() === "Summer 2026");
    await summer!.trigger("click");
    await flushPromises();

    expect(router.currentRoute.value.name).toBe("home");
    expect(router.currentRoute.value.query).toEqual({ year: "2026", term: "S" });
  });

  it("lands on the start page rather than carrying a course across", async () => {
    // A course belongs to the semester in its own URL; keeping the slug would ask for
    // a course that does not exist in the semester just chosen.
    router.push("/course/2026/W/eidi");
    await router.isReady();

    const wrapper = await render();
    await wrapper.get("button").trigger("click");
    await wrapper
      .findAll("button")
      .find((b) => b.text() === "Summer 2026")!
      .trigger("click");
    await flushPromises();

    expect(router.currentRoute.value.path).toBe("/");
  });
});

describe("the mobile sidebar", () => {
  it("closes when a course is opened, so the page is not left covered", async () => {
    const sidenav = useSidenavStore();
    const semesters = useSemesterStore();
    await semesters.load();
    sidenav.toggle(true);

    const wrapper = await render({ public: courses(1, "Public") });
    await wrapper.get('a[href^="/course/"]').trigger("click");
    await flushPromises();

    expect(sidenav.open).toBe(false);
    expect(router.currentRoute.value.name).toBe("course");
  });
});
