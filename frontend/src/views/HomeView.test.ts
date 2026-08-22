import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter, type Router } from "vue-router";

import HomeView from "./HomeView.vue";
import { useAuthStore } from "@/stores/auth";
import { useCourseStore } from "@/stores/courses";

/**
 * The start page's job is choosing what to show from the listings it is handed. Those
 * rules came from mainContext in web/ts/components/main.ts and none of them is
 * type-checkable: showing the wrong courses under "Today" looks entirely plausible.
 */

const { fetchServerNotifications, fetchProgress } = vi.hoisted(() => ({
  fetchServerNotifications: vi.fn(),
  fetchProgress: vi.fn(),
}));

vi.mock("@/lib/notifications", () => ({ fetchServerNotifications }));
vi.mock("@/lib/streams", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/streams")>()),
  fetchProgress,
}));
// The layout has its own tests, and mounting it would start loading listings.
vi.mock("@/components/StartPageLayout.vue", () => ({
  default: { template: "<div><slot /></div>" },
}));

const HOUR = 60 * 60 * 1000;

function lecture(id: number, offsetMs: number) {
  const start = new Date(Date.now() + offsetMs);
  return {
    id,
    name: `Lecture ${id}`,
    start,
    end: new Date(start.getTime() + HOUR),
    duration: 3600,
    downloads: [],
  };
}

function course(id: number, name: string, extra: Record<string, unknown> = {}) {
  return {
    id,
    name,
    slug: name.toLowerCase(),
    semester: { year: 2026, term: "W" },
    pinned: false,
    isAdmin: false,
    downloadsEnabled: false,
    streams: [],
    ...extra,
  };
}

let router: Router;

async function render(listings: Record<string, unknown[]> = {}, signedIn = true) {
  const courses = useCourseStore();
  courses.userCourses = (listings.user ?? []) as never[];
  courses.publicCourses = (listings.public ?? []) as never[];
  courses.pinnedCourses = (listings.pinned ?? []) as never[];
  courses.liveStreams = (listings.live ?? []) as never[];
  courses.loaded = true;

  if (signedIn) {
    useAuthStore().user = {
      id: 1,
      name: "Hansi",
      settings: { greeting: "Servus", preferredName: "Hansi" },
    } as never;
  }

  const wrapper = mount(HomeView, { global: { plugins: [router] } });
  await flushPromises();
  return wrapper;
}

beforeEach(async () => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  fetchServerNotifications.mockResolvedValue([]);
  fetchProgress.mockResolvedValue(new Map());

  router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/", name: "home", component: { template: "<div />" } },
      { path: "/courses/mine", name: "my-courses", component: { template: "<div />" } },
      { path: "/course/:year/:term/:slug", name: "course", component: { template: "<div />" } },
    ],
  });
  await router.push("/");
  await router.isReady();
});

describe("Today", () => {
  it("shows a course whose next lecture is later today", async () => {
    const wrapper = await render({ user: [course(1, "Soon", { nextLecture: lecture(9, HOUR) })] });

    expect(wrapper.find("#live-today").exists()).toBe(true);
    expect(wrapper.find("#live-today").text()).toContain("Soon");
  });

  it("leaves out a lecture that has already started", async () => {
    // The old page took only lectures still ahead: a lecture in progress belongs
    // under Live, not as something to wait for.
    const wrapper = await render({
      user: [course(1, "Started", { nextLecture: lecture(9, -HOUR) })],
    });

    expect(wrapper.find("#live-today").exists()).toBe(false);
  });

  it("leaves out a lecture on another day", async () => {
    const wrapper = await render({
      user: [course(1, "Tomorrow", { nextLecture: lecture(9, 30 * HOUR) })],
    });

    expect(wrapper.find("#live-today").exists()).toBe(false);
  });

  it("never shows a public course the user is not enrolled in", async () => {
    const wrapper = await render({
      public: [course(2, "Public", { nextLecture: lecture(9, HOUR) })],
    });

    expect(wrapper.find("#live-today").exists()).toBe(false);
  });
});

describe("Recent VODs", () => {
  const withRecording = (id: number, name: string) =>
    course(id, name, { lastRecording: lecture(100 + id, -48 * HOUR) });

  it("draws on the user's own courses and anything they pinned", async () => {
    const wrapper = await render({
      user: [withRecording(1, "Mine")],
      pinned: [withRecording(2, "Pinned")],
      public: [withRecording(3, "Public")],
    });

    const text = wrapper.find("#recent-vods").text();
    expect(text).toContain("Mine");
    expect(text).toContain("Pinned");
    expect(text).not.toContain("Public");
  });

  it("does not list a pinned course twice when it is also the user's own", async () => {
    const both = withRecording(1, "Mine");
    const wrapper = await render({ user: [both], pinned: [both] });

    expect(wrapper.findAll("#recent-vods article.tum-live-stream")).toHaveLength(1);
  });

  it("falls back to the public listing for someone with no courses", async () => {
    const wrapper = await render({ public: [withRecording(3, "Public")] }, false);

    expect(wrapper.find("#recent-vods").text()).toContain("Public");
  });

  it("leaves out a course that has never been recorded", async () => {
    const wrapper = await render({ user: [course(1, "Never recorded")] });

    expect(wrapper.find("#recent-vods").exists()).toBe(false);
  });

  it("reveals ten at a time", async () => {
    const many = Array.from({ length: 14 }, (_, i) => withRecording(i + 1, `Course ${i + 1}`));
    const wrapper = await render({ user: many });

    expect(wrapper.findAll("#recent-vods article.tum-live-stream")).toHaveLength(10);

    await wrapper.get("#recent-vods button").trigger("click");
    expect(wrapper.findAll("#recent-vods article.tum-live-stream")).toHaveLength(14);
  });
});

describe("the greeting", () => {
  it("uses the user's chosen greeting and name", async () => {
    const wrapper = await render();

    expect(wrapper.get("#greeting").text()).toBe("Servus Hansi, nice to see you! 👋");
  });

  it("is absent for a visitor who is not signed in", async () => {
    const wrapper = await render({}, false);

    expect(wrapper.find("#greeting").exists()).toBe(false);
  });
});

describe("server notifications", () => {
  it("renders the administrator's markup, warnings styled as such", async () => {
    fetchServerNotifications.mockResolvedValue([
      { html: "<b>Down</b> at noon", warn: true },
      { html: "Hello", warn: false },
    ]);
    const wrapper = await render();

    const banners = wrapper.findAll(".tum-live-notification");
    expect(banners).toHaveLength(2);
    expect(banners[0].html()).toContain("<b>Down</b>");
    expect(banners[0].classes()).toContain("tum-live-notification-warn");
    expect(banners[1].classes()).toContain("tum-live-notification-info");
  });
});

describe("an empty semester", () => {
  it("says so once the listings have loaded", async () => {
    const wrapper = await render();

    expect(wrapper.text()).toContain("No streams, VODs or courses to show in this semester.");
  });

  it("says nothing while they are still loading", async () => {
    const courses = useCourseStore();
    const wrapper = mount(HomeView, { global: { plugins: [router] } });
    await flushPromises();
    courses.loaded = false;

    expect(wrapper.text()).not.toContain("No streams, VODs or courses");
  });
});
