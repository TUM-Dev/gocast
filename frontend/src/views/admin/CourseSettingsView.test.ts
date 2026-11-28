import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CourseAdmin } from "@/lib/course-admin";
import CourseSettingsView from "./CourseSettingsView.vue";
import { useCourseAdminStore } from "@/stores/course-admin";

const api = vi.hoisted(() => ({ updateCourseSettings: vi.fn() }));
vi.mock("@/lib/course-admin", async (orig) => ({ ...(await orig<typeof import("@/lib/course-admin")>()), ...api }));

const course: CourseAdmin = {
  id: 3,
  name: "Course",
  slug: "course",
  year: 2026,
  term: "W",
  visibility: "public",
  vodEnabled: false,
  downloadsEnabled: false,
  chatEnabled: false,
  anonymousChatEnabled: false,
  moderatedChatEnabled: false,
  vodChatEnabled: false,
  livePrivate: false,
  vodPrivate: false,
  lectureExpirationDays: 7,
  language: "",
  tumOnlineIdentifier: "",
};

beforeEach(() => {
  setActivePinia(createPinia());
  api.updateCourseSettings.mockReset().mockImplementation(async (_id, settings) => ({ ...course, ...settings }));
  useCourseAdminStore().replace(course);
});

describe("CourseSettingsView", () => {
  it("treats a cleared lecture-expiration field as zero and saves it", async () => {
    const w = mount(CourseSettingsView, {
      global: {
        stubs: {
          CourseActionsSection: true,
          CourseAdminsSection: true,
          CourseAuditSection: true,
          CourseIntegrationGrantsSection: true,
          LectureHallSettingsSection: true,
        },
      },
    });

    await w.get("#lecture-expiration-days").setValue("");
    await w.get("form").trigger("submit");
    await flushPromises();

    expect(api.updateCourseSettings).toHaveBeenCalledWith(3, { lectureExpirationDays: 0 });
    expect(w.get('[role="status"]').text()).toBe("Settings saved.");
  });
});
