import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import {
  StreamProtocol,
  createLectureHall,
  deleteLectureHall,
  fetchLectureHalls,
  updateLectureHall,
  type LectureHallInput,
} from "./lecture-halls";

let fetchMock: ReturnType<typeof vi.fn>;

/** Answers the token request, then the given body for everything else. */
function respondWith(body: unknown, status = 200): void {
  fetchMock.mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(
        new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 }),
      );
    }
    return Promise.resolve(new Response(JSON.stringify(body), { status }));
  });
}

const input: LectureHallInput = {
  name: "FMI_HS1",
  streamProtocol: StreamProtocol.RTSP,
  combIp: "",
  presIp: "",
  camIp: "rtsp://0.0.0.0/cam",
  cameraIp: "",
  pwrCtrlIp: "",
};

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchLectureHalls", () => {
  it("reads a hall's fields off the wire", async () => {
    respondWith({
      lectureHalls: [
        {
          id: 1,
          name: "FMI_HS1",
          streamProtocol: 1,
          camIp: "rtsp://0.0.0.0/cam",
        },
      ],
    });

    const [hall] = await fetchLectureHalls();

    expect(hall.id).toBe(1);
    expect(hall.name).toBe("FMI_HS1");
    expect(hall.streamProtocol).toBe(StreamProtocol.RTSP);
    expect(hall.camIp).toBe("rtsp://0.0.0.0/cam");
  });

  it("reads an empty list as no halls rather than as a failure", async () => {
    // protojson omits an empty repeated field, so the response body is `{}`.
    respondWith({});

    await expect(fetchLectureHalls()).resolves.toEqual([]);
  });

  it("asks the admin endpoint", async () => {
    respondWith({});

    await fetchLectureHalls();

    const call = fetchMock.mock.calls.find(([url]) => !String(url).endsWith("/auth/token"));
    expect(call?.[0]).toBe("/api/v2/admin/lecture-halls");
  });
});

describe("createLectureHall", () => {
  it("posts the given fields and returns the created hall", async () => {
    respondWith({ id: 5, name: "FMI_HS1", streamProtocol: 1, camIp: "rtsp://0.0.0.0/cam" });

    const created = await createLectureHall(input);

    expect(created.id).toBe(5);
    const call = fetchMock.mock.calls.find(([url]) => !String(url).endsWith("/auth/token"));
    const [url, init] = call as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/lecture-halls");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toMatchObject({ name: "FMI_HS1" });
  });
});

describe("updateLectureHall", () => {
  it("patches the hall by id", async () => {
    respondWith({ id: 3, name: "New Name", streamProtocol: 2 });

    const updated = await updateLectureHall(3, { ...input, name: "New Name" });

    expect(updated.name).toBe("New Name");
    const call = fetchMock.mock.calls.find(([url]) => !String(url).endsWith("/auth/token"));
    const [url, init] = call as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/lecture-halls/3");
    expect(init.method).toBe("PATCH");
  });
});

describe("deleteLectureHall", () => {
  it("deletes by id", async () => {
    respondWith({});

    await deleteLectureHall(3);

    const call = fetchMock.mock.calls.find(([url]) => !String(url).endsWith("/auth/token"));
    const [url, init] = call as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/lecture-halls/3");
    expect(init.method).toBe("DELETE");
  });
});
