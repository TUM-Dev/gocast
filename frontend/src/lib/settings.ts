/**
 * User settings, as exposed by GET /api/v2/users/me and PATCH /api/v2/users/settings.
 *
 * The wire format comes from the generated schemas in src/gen. What remains is the
 * storage format: each value is an opaque string that its getter in model/user.go
 * interprets its own way, which the encode/decode helpers keep out of the components.
 */

import { create } from "@bufbuild/protobuf";

import {
  GetUserResponseSchema,
  UpdateUserSettingsRequestSchema,
  UpdateUserSettingsResponseSchema,
  UserSettingType,
  type User,
} from "@/gen/server/apiv2_pb";
import { apiGetMessage, apiPatchMessage } from "./api";

export { UserSettingType };

export interface PlaybackSpeed {
  speed: number;
  enabled: boolean;
}

export interface AutoSkipSetting {
  enabled: boolean;
}

export type LectureView = "Presentation" | "Camera" | "Split" | "Combined";

export interface UserSettings {
  preferredName: string;
  greeting: string;
  playbackSpeeds: PlaybackSpeed[];
  customSpeeds: number[];
  seekingTime: number;
  autoSkip: boolean;
  lectureView: LectureView;
}

export interface CurrentUser {
  id: number;
  name: string;
  email: string;
  role: number;
  settings: UserSettings;
}

/**
 * Defaults for a user who has never saved playback speeds.
 *
 * Must stay identical to model.defaultPlaybackSpeeds in model/user.go, entries and
 * enabled flags alike: the first change a user makes is saved as the whole array, so
 * anything wrong here is written to their account the moment they toggle a speed.
 */
const DEFAULT_PLAYBACK_SPEEDS: readonly PlaybackSpeed[] = [
  { speed: 0.25, enabled: false },
  { speed: 0.5, enabled: true },
  { speed: 0.75, enabled: true },
  { speed: 1, enabled: true },
  { speed: 1.25, enabled: true },
  { speed: 1.5, enabled: true },
  { speed: 1.75, enabled: true },
  { speed: 2, enabled: true },
  { speed: 2.5, enabled: false },
  { speed: 3, enabled: false },
  { speed: 3.5, enabled: false },
];

/** A fresh copy, so a component mutating its settings cannot alter the defaults. */
function defaultPlaybackSpeeds(): PlaybackSpeed[] {
  return DEFAULT_PLAYBACK_SPEEDS.map((entry) => ({ ...entry }));
}

/**
 * Settings whose value is stored as a bare string rather than as JSON.
 *
 * GetPreferredName, GetPreferredGreeting and GetPreferredView in model/user.go return
 * `setting.Value` verbatim; the others run it through json.Unmarshal. Encoding a name
 * as JSON would store the quotes as part of the name. SEEKING_TIME needs no special
 * handling because a JSON-encoded number is identical to a plain one.
 */
const RAW_STRING_TYPES = new Set<UserSettingType>([
  UserSettingType.PREFERRED_NAME,
  UserSettingType.GREETING,
  UserSettingType.LECTURE_VIEW,
]);

/** Encodes a value into the storage format the corresponding Go getter expects. */
function encode(type: UserSettingType, value: unknown): string {
  return RAW_STRING_TYPES.has(type) ? String(value) : JSON.stringify(value);
}

/**
 * Reads a bare-string setting. Values mistakenly stored JSON-encoded are unwrapped so
 * affected rows heal on the next read instead of displaying their quotes.
 */
function decodeRawString(raw: string | undefined, fallback: string): string {
  if (raw === undefined || raw === "") {
    return fallback;
  }
  if (raw.length > 1 && raw.startsWith('"') && raw.endsWith('"')) {
    try {
      const unwrapped = JSON.parse(raw);
      if (typeof unwrapped === "string") {
        return unwrapped;
      }
    } catch {
      // Not JSON after all — fall through and use the value as stored.
    }
  }
  return raw;
}

/** Unwraps a JSON-encoded setting value, tolerating values stored unencoded. */
function decode<T>(raw: string | undefined, fallback: T): T {
  if (raw === undefined || raw === "") {
    return fallback;
  }
  try {
    return JSON.parse(raw) as T;
  } catch {
    return raw as unknown as T;
  }
}

function parseSettings(user: User): UserSettings {
  const byType = new Map<UserSettingType, string>();
  for (const setting of user.settings) {
    byType.set(setting.type, setting.value);
  }

  // Fallbacks mirror the getters in model/user.go for users who have set nothing.
  return {
    preferredName: decodeRawString(byType.get(UserSettingType.PREFERRED_NAME), user.name),
    greeting: decodeRawString(byType.get(UserSettingType.GREETING), "Moin"),
    playbackSpeeds: decode(
      byType.get(UserSettingType.CUSTOM_PLAYBACK_SPEEDS),
      defaultPlaybackSpeeds(),
    ),
    customSpeeds: decode(byType.get(UserSettingType.USER_DEFINED_SPEEDS), [] as number[]),
    seekingTime: decode(byType.get(UserSettingType.SEEKING_TIME), 10),
    autoSkip: decode<AutoSkipSetting>(byType.get(UserSettingType.AUTO_SKIP), { enabled: false })
      .enabled,
    lectureView: decodeRawString(
      byType.get(UserSettingType.LECTURE_VIEW),
      "Combined",
    ) as LectureView,
  };
}

export async function fetchCurrentUser(): Promise<CurrentUser> {
  const res = await apiGetMessage(GetUserResponseSchema, "/users/me");
  if (!res.user) {
    throw new Error("the API returned no user");
  }

  return {
    id: res.user.id,
    name: res.user.name,
    email: res.user.email,
    role: res.user.role,
    settings: parseSettings(res.user),
  };
}

/** Persists one setting. The API replaces per type, so send the complete value. */
export async function updateSetting(type: UserSettingType, value: unknown): Promise<void> {
  const request = create(UpdateUserSettingsRequestSchema, {
    userSettings: [{ type, value: encode(type, value) }],
  });

  await apiPatchMessage(
    UpdateUserSettingsRequestSchema,
    UpdateUserSettingsResponseSchema,
    "/users/settings",
    request,
  );
}

/** Rounds a speed to two decimals and clamps it to the range the player accepts. */
export function sanitizeSpeed(input: number | string): number | null {
  const parsed = typeof input === "number" ? input : Number.parseFloat(input);
  if (Number.isNaN(parsed)) {
    return null;
  }
  const rounded = Math.round(parsed * 100) / 100;
  if (rounded < 0.25 || rounded > 5) {
    return null;
  }
  return rounded;
}
