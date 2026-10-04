import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter } from "vue-router";

import ServerStatsView from "./admin/ServerStatsView.vue";
import { useAuthStore } from "@/stores/auth";

const { Chart } = vi.hoisted(() => ({
  Chart: vi.fn(function (this: { destroy: () => void }, _canvas: HTMLCanvasElement, _config: unknown) {
    this.destroy = vi.fn();
  }),
}));

// jsdom has no canvas to draw on; what matters is which canvases a chart was built on.
vi.mock("chart.js/auto", () => ({ Chart }));

vi.mock("@/lib/server-stats", async () => {
  const actual = await vi.importActual<typeof import("@/lib/server-stats")>("@/lib/server-stats");
  const series = [{ x: "2026-01", y: 3 }];
  return {
    ...actual,
    fetchServerStats: vi.fn().mockResolvedValue({
      numStudents: 8,
      vodViews: 0,
      liveViews: 0,
      numLectures: 7,
      activityLive: series,
      activityVod: series,
      hourly: series,
      weekdays: series,
      allDays: series,
    }),
  };
});

beforeEach(() => {
  setActivePinia(createPinia());
  Chart.mockClear();
});

describe("ServerStatsView", () => {
  it("draws a chart on each canvas once the statistics have loaded", async () => {
    const auth = useAuthStore();
    auth.user = {
      id: 1,
      name: "Test",
      email: "t@example.com",
      role: 1,
      permissions: ["server.administer"],
    } as never;
    auth.loaded = true;

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/:any(.*)", component: { template: "<div />" } }],
    });
    const wrapper = mount(ServerStatsView, { global: { plugins: [router] } });
    await flushPromises();

    const canvases = wrapper.findAll("canvas").map((c) => c.element);
    expect(canvases).toHaveLength(5);
    // A canvas that exists but never had a chart built on it renders blank, so count
    // the charts, not the canvases.
    expect(Chart).toHaveBeenCalledTimes(5);
    expect(Chart.mock.calls.map((call) => call[0])).toEqual(canvases);
  });
});
