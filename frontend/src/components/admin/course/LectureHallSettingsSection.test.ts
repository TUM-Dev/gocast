import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api";
import { CourseSourceMode, type LectureHallSetting } from "@/lib/course-admin";
import LectureHallSettingsSection from "./LectureHallSettingsSection.vue";

const api = vi.hoisted(() => ({ fetchLectureHallSettings: vi.fn(), updateLectureHallSettings: vi.fn() }));
vi.mock("@/lib/course-admin", async (orig) => ({ ...(await orig<typeof import("@/lib/course-admin")>()), ...api }));

const hall = (over: Partial<LectureHallSetting> = {}): LectureHallSetting => ({
  lectureHallId: 4,
  lectureHallName: "HS1",
  sourceMode: CourseSourceMode.COMBINED,
  selectedPresetId: 0,
  presets: [
    { presetId: 1, name: "Board", image: "board.jpg", isDefault: true },
    { presetId: 2, name: "Lectern", image: "", isDefault: false },
    { presetId: 3, name: "gelöscht", image: "old.jpg", isDefault: false },
  ],
  ...over,
});

async function section(halls: LectureHallSetting[] = [hall()]) {
  api.fetchLectureHallSettings.mockResolvedValue(halls);
  const w = mount(LectureHallSettingsSection, { props: { courseId: 3 } });
  await flushPromises();
  return w;
}

const tile = (hallId: number, presetId: number) => `[data-testid=preset-tile-${hallId}-${presetId}]`;
const saveButton = "button[type=button]";

beforeEach(() => {
  api.fetchLectureHallSettings.mockReset();
  api.updateLectureHallSettings.mockReset().mockImplementation(async (_, halls) => halls.map((h: LectureHallSetting) => ({ ...h })));
});

describe("LectureHallSettingsSection", () => {
  it("offers tiles instead of a preset dropdown", async () => {
    const w = await section();
    expect(w.findAll("select")).toHaveLength(1);
    expect(w.get(tile(4, 0)).text()).toBe("Hall default (Board)");
    expect(w.get(`${tile(4, 0)} img`).attributes("src")).toBe("/public/board.jpg");
    expect(w.get(`${tile(4, 2)} img`).attributes("src")).toBe("/public/noPreset.jpg");
  });

  it("hides a deleted preset unless it is the chosen one", async () => {
    expect((await section()).find(tile(4, 3)).exists()).toBe(false);
    expect((await section([hall({ selectedPresetId: 3 })])).find(tile(4, 3)).exists()).toBe(true);
  });

  it("shows no tiles for a hall without presets", async () => {
    const w = await section([hall({ presets: [] })]);
    expect(w.find("fieldset").exists()).toBe(false);
    expect(w.text()).toContain("HS1");
  });

  it("saves the selection only on Save, with the other hall as it was", async () => {
    const other = hall({ lectureHallId: 5, lectureHallName: "HS2", selectedPresetId: 1 });
    const w = await section([hall(), other]);
    expect(w.get(saveButton).attributes("disabled")).toBeDefined();
    await w.get(`${tile(4, 2)} input`).setValue(true);
    expect(api.updateLectureHallSettings).not.toHaveBeenCalled();
    expect((w.get(`${tile(5, 1)} input`).element as HTMLInputElement).checked).toBe(true);

    await w.get(saveButton).trigger("click");
    await flushPromises();
    const [courseId, sent] = api.updateLectureHallSettings.mock.calls[0];
    expect(courseId).toBe(3);
    expect(sent.map((h: LectureHallSetting) => [h.lectureHallId, h.selectedPresetId])).toEqual([[4, 2], [5, 1]]);
    expect(w.get("[role=status]").text()).toBe("Saved.");
    expect(w.get(saveButton).attributes("disabled")).toBeDefined();
  });

  it("reports a failed save and keeps the selection", async () => {
    api.updateLectureHallSettings.mockRejectedValue(new ApiError(403, "not allowed"));
    const w = await section();
    await w.get(`${tile(4, 2)} input`).setValue(true);
    await w.get(saveButton).trigger("click");
    await flushPromises();
    expect(w.get("[role=alert]").text()).toBe("not allowed");
    expect((w.get(`${tile(4, 2)} input`).element as HTMLInputElement).checked).toBe(true);
  });
});
