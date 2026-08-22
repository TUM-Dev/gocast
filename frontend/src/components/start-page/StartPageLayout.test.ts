import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter, type Router } from "vue-router";

import StartPageLayout from "./StartPageLayout.vue";
import { useSidenavStore } from "@/stores/sidenav";

/**
 * The layout is where the start page decides which semester it is showing and loads
 * the listings for it, once, on behalf of both the sidebar and the view.
 */

const { fetchSemesters, load } = vi.hoisted(() => ({
  fetchSemesters: vi.fn(),
  load: vi.fn(),
}));

vi.mock("@/lib/semesters", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/semesters")>()),
  fetchSemesters,
}));

// The sidebar has its own tests; here it is only noise.
vi.mock("./StartPageSidenav.vue", () => ({ default: { template: "<nav />" } }));

const current = { year: 2026, term: "W" } as const;
const blank = { template: "<div />" };

let router: Router;

function makeRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/", name: "home", component: blank },
      { path: "/course/:year/:term/:slug", name: "course", component: blank },
    ],
  });
}

async function render() {
  const wrapper = mount(StartPageLayout, {
    global: { plugins: [router] },
    slots: { default: "<p>content</p>" },
  });
  await flushPromises();
  return wrapper;
}

beforeEach(async () => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  fetchSemesters.mockResolvedValue({ current, all: [current, { year: 2026, term: "S" }] });
  router = makeRouter();
  await router.push("/");
  await router.isReady();
});

vi.mock("@/stores/courses", () => ({ useCourseStore: () => ({ load }) }));

describe("loading the listings", () => {
  it("waits for the current semester rather than fetching for none and again for it", async () => {
    await render();

    // Not called with undefined first: that would be two sets of requests per visit.
    expect(load).toHaveBeenCalledTimes(1);
    expect(load).toHaveBeenCalledWith(current);
  });

  it("uses the semester in the URL over the current one", async () => {
    await router.push("/?year=2025&term=S");
    await render();

    expect(load).toHaveBeenCalledWith({ year: 2025, term: "S" });
  });

  it("reads the semester from the path on a course page", async () => {
    // The course route carries it as /course/:year/:term/:slug, not as a query, and
    // the sidebar beside it has to show that semester's courses.
    await router.push("/course/2024/S/eidi");
    await render();

    expect(load).toHaveBeenCalledWith({ year: 2024, term: "S" });
  });

  it("loads again when the semester changes", async () => {
    await render();
    await router.push("/?year=2025&term=S");
    await flushPromises();

    expect(load).toHaveBeenCalledTimes(2);
    expect(load).toHaveBeenLastCalledWith({ year: 2025, term: "S" });
  });
});

describe("the mobile sidebar", () => {
  it("closes on navigation, so it does not cover the page just opened", async () => {
    const sidenav = useSidenavStore();
    await render();
    sidenav.toggle(true);

    await router.push("/course/2026/W/eidi");
    await flushPromises();

    expect(sidenav.open).toBe(false);
  });
});
