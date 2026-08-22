import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter, type Router } from "vue-router";

import CourseView from "./CourseView.vue";
import { ApiError } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";

/**
 * The course page sorts one list of lectures into three — live, scheduled, recorded —
 * and decides which controls the caller gets. Both are silent when wrong: a scheduled
 * lecture listed as a recording links to a video that does not exist.
 */

const { fetchCourse, fetchProgress, setWatched, redirectToLogin } = vi.hoisted(() => ({
  fetchCourse: vi.fn(),
  fetchProgress: vi.fn(),
  setWatched: vi.fn(),
  redirectToLogin: vi.fn(),
}));

vi.mock("@/lib/courses", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/courses")>()),
  fetchCourse,
}));
vi.mock("@/lib/streams", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/streams")>()),
  fetchProgress,
  setWatched,
}));
vi.mock("@/stores/auth", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/stores/auth")>()),
  redirectToLogin,
}));
vi.mock("@/components/start-page/StartPageLayout.vue", () => ({
  default: { template: "<div><slot /></div>" },
}));

const HOUR = 60 * 60 * 1000;

function lecture(id: number, offsetMs: number, extra: Record<string, unknown> = {}) {
  const start = new Date(Date.now() + offsetMs);
  return {
    id,
    name: `Lecture ${id}`,
    start,
    end: new Date(start.getTime() + HOUR),
    duration: 3600,
    downloads: [],
    isPubliclyVisible: true,
    recording: false,
    isPlanned: false,
    isComingUp: false,
    hlsUrl: "",
    roomCode: "",
    ...extra,
  };
}

function makeCourse(streams: unknown[], extra: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: "Introduction to Informatics",
    slug: "eidi",
    semester: { year: 2026, term: "W" },
    visibility: "public",
    pinned: false,
    isAdmin: false,
    downloadsEnabled: false,
    streams,
    ...extra,
  };
}

let router: Router;

async function render(signedIn = true) {
  if (signedIn) useAuthStore().user = { id: 1, name: "Hansi", role: 3 } as never;

  const wrapper = mount(CourseView, {
    props: { year: "2026", term: "W", slug: "eidi" },
    global: { plugins: [router] },
  });
  await flushPromises();
  return wrapper;
}

beforeEach(async () => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  localStorage.clear();
  fetchProgress.mockResolvedValue(new Map());

  router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/", name: "home", component: { template: "<div />" } },
      { path: "/course/:year/:term/:slug", name: "course", component: CourseView, props: true },
    ],
  });
  await router.push("/course/2026/W/eidi");
  await router.isReady();
});

describe("sorting the lectures", () => {
  it("puts each lecture where it belongs", async () => {
    fetchCourse.mockResolvedValue(
      makeCourse([
        lecture(1, -48 * HOUR, { recording: true }),
        lecture(2, 48 * HOUR, { isPlanned: true }),
        lecture(3, HOUR, { isComingUp: true }),
      ]),
    );
    const wrapper = await render();

    expect(wrapper.text()).toContain("VODs");
    expect(wrapper.text()).toContain("Scheduled");
    expect(wrapper.text()).toContain("Join waiting room");
  });

  it("leaves out the sections with nothing in them", async () => {
    fetchCourse.mockResolvedValue(makeCourse([lecture(1, -48 * HOUR, { recording: true })]));
    const wrapper = await render();

    expect(wrapper.text()).toContain("VODs");
    expect(wrapper.text()).not.toContain("Scheduled");
    expect(wrapper.text()).not.toContain("Join waiting room");
  });

  it("shows three scheduled lectures before offering the rest", async () => {
    const planned = Array.from({ length: 5 }, (_, i) =>
      lecture(i + 1, (i + 1) * 24 * HOUR, { isPlanned: true }),
    );
    fetchCourse.mockResolvedValue(makeCourse(planned));
    const wrapper = await render();

    expect(wrapper.findAll("article.rounded-lg")).toHaveLength(3);

    const showAll = wrapper.findAll("button").find((b) => b.text() === "Show all");
    await showAll!.trigger("click");

    expect(wrapper.findAll("article.rounded-lg")).toHaveLength(5);
    expect(wrapper.findAll("button").some((b) => b.text() === "Show all")).toBe(false);
  });
});

describe("what the caller may do", () => {
  beforeEach(() => {
    fetchCourse.mockResolvedValue(makeCourse([lecture(1, -HOUR, { recording: true })]));
  });

  it("offers the pin and the watched filter only to a signed-in caller", async () => {
    const anonymous = await render(false);
    expect(anonymous.text()).not.toContain("Pin");
    expect(anonymous.text()).not.toContain("Hide watched");

    setActivePinia(createPinia());
    const signedIn = await render();
    expect(signedIn.text()).toContain("Pin");
    expect(signedIn.text()).toContain("Hide watched");
  });

  it("offers the calendar feed to everyone", async () => {
    const wrapper = await render(false);

    expect(wrapper.get('a[download]').attributes("href")).toBe(
      "/api/download_ics/2026/W/eidi/events.ics",
    );
  });

  it("hides the admin link from a caller who administers neither", async () => {
    const wrapper = await render();

    expect(wrapper.find('a[href="/admin/course/1"]').exists()).toBe(false);
  });

  it("shows it to an administrator of this course", async () => {
    fetchCourse.mockResolvedValue(makeCourse([], { isAdmin: true }));
    const wrapper = await render();

    expect(wrapper.find('a[href="/admin/course/1"]').exists()).toBe(true);
  });

  it("shows it to a global administrator of any course", async () => {
    useAuthStore().user = { id: 1, name: "Root", role: 1 } as never;
    fetchCourse.mockResolvedValue(makeCourse([]));
    const wrapper = mount(CourseView, {
      props: { year: "2026", term: "W", slug: "eidi" },
      global: { plugins: [router] },
    });
    await flushPromises();

    expect(wrapper.find('a[href="/admin/course/1"]').exists()).toBe(true);
  });
});

describe("a course that will not load", () => {
  it("sends an unauthorised caller to sign in", async () => {
    fetchCourse.mockRejectedValue(new ApiError(401, "unauthorized"));
    await render(false);

    expect(redirectToLogin).toHaveBeenCalled();
  });

  it("sends a signed-in caller who is refused back to the start page", async () => {
    // A 401 for someone already signed in means they may not have this course, not
    // that they need to sign in — a login page they have been through explains nothing.
    fetchCourse.mockRejectedValue(new ApiError(401, "unauthorized"));
    await render();

    expect(redirectToLogin).not.toHaveBeenCalled();
    expect(router.currentRoute.value.name).toBe("home");
  });

  it("sends everything else back to a page that exists", async () => {
    // A renamed slug or a deleted course, where signing in would not help.
    fetchCourse.mockRejectedValue(new ApiError(404, "not found"));
    await render();

    expect(router.currentRoute.value.name).toBe("home");
    expect(router.currentRoute.value.query).toEqual({ year: "2026", term: "W" });
  });
});

describe("watch progress", () => {
  it("asks for none when nobody is signed in", async () => {
    fetchCourse.mockResolvedValue(makeCourse([lecture(1, -HOUR, { recording: true })]));
    await render(false);

    expect(fetchProgress).not.toHaveBeenCalled();
  });

  it("asks only for the recordings", async () => {
    fetchCourse.mockResolvedValue(
      makeCourse([
        lecture(1, -HOUR, { recording: true }),
        lecture(2, 48 * HOUR, { isPlanned: true }),
      ]),
    );
    await render();

    expect(fetchProgress).toHaveBeenCalledWith([1]);
  });
});
