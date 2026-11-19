import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";

import { OfflineLectureSchema, OfflineLectureState, OfflineLicenseSchema } from "@/gen/server/apiv2_pb";
import { OfflineError, recordFromLecture, waitUntilReady } from "./download";

const lecture = (state: OfflineLectureState, extra: MessageInitShape<typeof OfflineLectureSchema> = {}) =>
  create(OfflineLectureSchema, { packageId: 9, state, ...extra });

describe("waitUntilReady", () => {
  it("polls while the runner prepares, then returns the licence", async () => {
    const answers = [
      lecture(OfflineLectureState.PREPARING),
      lecture(OfflineLectureState.PREPARING),
      lecture(OfflineLectureState.READY, { license: create(OfflineLicenseSchema, { playlistUrl: "https://edge/x" }) }),
    ];
    let waits = 0;
    const got = await waitUntilReady(9, async () => answers.shift()!, async () => waits++);
    expect(got.license?.playlistUrl).toBe("https://edge/x");
    expect(waits).toBe(2);
  });

  it("gives up for good on a failed or revoked package", async () => {
    for (const state of [OfflineLectureState.FAILED, OfflineLectureState.REVOKED]) {
      const err = await waitUntilReady(9, async () => lecture(state, { error: "no" }), async () => {}).catch((e) => e);
      expect(err).toBeInstanceOf(OfflineError);
      expect((err as OfflineError).final).toBe(true);
    }
  });
});

describe("recordFromLecture", () => {
  it("starts as preparing, not completed, with nothing downloaded", () => {
    const r = recordFromLecture(lecture(OfflineLectureState.PREPARING, { startOffset: 30 }), 1000);
    expect(r).toMatchObject({ packageId: 9, state: "preparing", downloadedSegments: 0, completed: false, startOffset: 30, savedAt: 1000 });
  });
});
