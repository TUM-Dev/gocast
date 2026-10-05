import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter } from "vue-router";

import LectureHallsView from "./admin/LectureHallsView.vue";
import { useAuthStore } from "@/stores/auth";

const { hall } = vi.hoisted(() => ({
  hall: {
    id: 1,
    name: "Hall 1",
    streamProtocol: 1,
    combIp: "",
    presIp: "",
    camIp: "",
    cameraIp: "10.0.0.1",
    pwrCtrlIp: "",
    cameraPresets: [{ lectureHallId: 1, presetId: 7, name: "Main", image: "", isDefault: false }],
  },
}));

vi.mock("@/lib/lecture-halls", async () => {
  const actual = await vi.importActual<typeof import("@/lib/lecture-halls")>("@/lib/lecture-halls");
  return {
    ...actual,
    fetchLectureHalls: vi.fn().mockResolvedValue([hall]),
  };
});

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/admin/lecture-halls", name: "admin-lecture-halls", component: { template: "<div />" } },
      { path: "/admin/lecture-halls/new", name: "admin-lecture-halls-new", component: { template: "<div />" } },
      { path: "/admin/runners", name: "admin-runners", component: { template: "<div />" } },
      { path: "/admin/users", name: "admin-users", component: { template: "<div />" } },
      { path: "/admin/info-pages", name: "admin-info-pages", component: { template: "<div />" } },
    ],
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
});

describe("LectureHallsView", () => {
  it("keeps preset controls available on touch devices as well as on hover", async () => {
    const auth = useAuthStore();
    auth.user = {
      id: 1,
      name: "Test",
      email: "t@example.com",
      role: 1,
      permissions: ["server.administer"],
    } as never;
    auth.loaded = true;

    const wrapper = mount(LectureHallsView, { global: { plugins: [makeRouter()] } });
    await flushPromises();

    const snapshotButton = wrapper.find('button[title="Take new snapshot"]');
    expect(snapshotButton.exists()).toBe(true);
    expect(snapshotButton.classes()).toContain("opacity-100");
    expect(snapshotButton.classes()).toContain("sm:opacity-0");
  });
});
