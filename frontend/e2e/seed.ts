/**
 * The database the browser tests run against: `tum-live-starter.sql`, loaded as-is and
 * migrated forward by the server on boot.
 *
 * Everything here is a restatement of that file, so the two can be checked against
 * each other by eye. Nothing is created by the suite — the fixture is the dump.
 *
 * Reset it before a run:
 *
 *   make e2e_db
 */

/** Every seeded account shares this password; the dump stores one argon2 hash. */
export const password = "password";

export interface SeedUser {
  /** The `email` column, which is what the login form takes as the username. */
  username: string;
  name: string;
  /** model.User.Role: 1 admin, 2 lecturer, 4 student. */
  role: number;
}

/**
 * The six accounts in the dump.
 *
 * `admin` holds the wildcard permission, so it administers every course. The two
 * lecturers administer only what `course_admins` grants them, and the three students
 * are enrolled only in what `course_users` grants them.
 */
export const users = {
  admin: { username: "admin", name: "Anja Admin", role: 1 },
  // course_admins: (1, 2), (3, 2) — Einführung Brauereiwesen and Praktikum: Golang.
  prof1: { username: "prof1", name: "Peter Prof", role: 2 },
  // course_admins: (1, 3), (2, 3) — Einführung Brauereiwesen and Spieleentwicklung.
  prof2: { username: "prof2", name: "Pauline Prof", role: 2 },
  // course_users: (1, 4), (2, 4)
  studi1: { username: "studi1", name: "Stephanie Studi", role: 4 },
  // course_users: (1, 5), (2, 5)
  studi2: { username: "studi2", name: "Sven Studi", role: 4 },
  // course_users: (1, 6), (3, 6)
  studi3: { username: "studi3", name: "Sandra Studi", role: 4 },
} as const satisfies Record<string, SeedUser>;

export type UserKey = keyof typeof users;

/**
 * The five courses, by the slug their URLs use — one for each of the four
 * visibilities, and a second public one in another semester.
 */
export const courses = {
  brauereiwesen: { name: "Einführung Brauereiwesen", visibility: "public", year: 2022, term: "S" },
  games101: { name: "Spieleentwicklung für Dummies", visibility: "loggedin", year: 2022, term: "S" },
  godev: { name: "Praktikum: Golang", visibility: "public", year: 2021, term: "W" },
  bierkunde: { name: "Fortgeschrittene Bierkunde", visibility: "enrolled", year: 2022, term: "S" },
  geheim: { name: "Geheime Vorlesung", visibility: "hidden", year: 2022, term: "S" },
} as const;

export type CourseKey = keyof typeof courses;

/** The semesters that have courses, as the start page addresses them. */
export const semesters = {
  summer2022: { year: 2022, term: "S", query: "?year=2022&term=S" },
  winter2021: { year: 2021, term: "W", query: "?year=2021&term=W" },
} as const;

/**
 * Which courses each caller is offered, per semester.
 *
 * `listed` is the public listing: public courses for everyone, plus the logged-in-only
 * ones for anyone signed in. `enrolled` is the caller's own — enrolment for students,
 * the courses they administer for lecturers, and everything for an admin.
 */
export const expected = {
  summer2022: {
    listed: {
      anonymous: ["brauereiwesen"],
      // games101 is `loggedin`, so it appears for every signed-in caller regardless
      // of whether they have anything to do with it.
      signedIn: ["brauereiwesen", "games101"],
    },
    enrolled: {
      // Every course of the semester, by the wildcard permission.
      admin: ["brauereiwesen", "games101", "bierkunde", "geheim"],
      // Administers brauereiwesen, godev and bierkunde; godev is another semester.
      prof1: ["brauereiwesen", "bierkunde"],
      prof2: ["brauereiwesen", "games101", "geheim"],
      studi1: ["brauereiwesen", "games101", "bierkunde"],
      studi2: ["brauereiwesen", "games101", "geheim"],
      // Enrolled in brauereiwesen and godev; godev is another semester.
      studi3: ["brauereiwesen"],
    },
  },
  winter2021: {
    listed: {
      anonymous: ["godev"],
      // Nothing logged-in-only in this semester, so both callers see the same.
      signedIn: ["godev"],
    },
    enrolled: {
      admin: ["godev"],
      prof1: ["godev"],
      prof2: [],
      studi1: [],
      studi2: [],
      studi3: ["godev"],
    },
  },
} as const satisfies Record<
  string,
  { listed: { anonymous: CourseKey[]; signedIn: CourseKey[] }; enrolled: Record<UserKey, CourseKey[]> }
>;

/**
 * Who administers each course, which decides who is offered the admin link on it.
 * The admin account is left out: it reaches every course through its role instead.
 */
export const administers: Record<CourseKey, UserKey[]> = {
  brauereiwesen: ["prof1", "prof2"],
  games101: ["prof2"],
  godev: ["prof1"],
  bierkunde: ["prof1"],
  geheim: ["prof2"],
};

/**
 * Who may open each course by its own URL, which is not the same question as who is
 * shown it in a listing.
 *
 * `hidden` is the one that comes apart: geheim is in nobody's public listing, but a
 * direct link opens it for anyone, including a visitor who is not signed in. That is
 * the whole difference between unlisted and private, and it is easy to get wrong in
 * the direction of hiding too much or too little.
 */
export const canOpen: Record<CourseKey, "everyone" | UserKey[]> = {
  brauereiwesen: "everyone",
  godev: "everyone",
  geheim: "everyone",
  // `loggedin`: any account will do, but not an anonymous visitor.
  games101: ["admin", "prof1", "prof2", "studi1", "studi2", "studi3"],
  // `enrolled`: only the student enrolled in it and the lecturer administering it.
  bierkunde: ["admin", "prof1", "studi1"],
};

/**
 * The lectures that are live, and who is shown them.
 *
 * The live listing is filtered by the listing rule, not the reachability one, so the
 * hidden course's lecture reaches only the people who administer it — not even the
 * student enrolled in it. That distinction is the reason for the second entry.
 *
 * Both run from half an hour before the dump was loaded until three hours after it:
 * the server reaps a stream back to not-live once its end has passed, so a livestream
 * only stays live for as long as the fixture is fresh.
 */
export const live = [
  { course: "games101", lecture: "VL 1: Livestream", shownTo: "signedIn" },
  { course: "geheim", lecture: "VL 1: Interne Vorführung", shownTo: ["admin", "prof2"] },
] as const satisfies readonly {
  course: CourseKey;
  lecture: string;
  shownTo: "signedIn" | readonly UserKey[];
}[];

/**
 * The recorded lectures of each course, newest first, which is the order the course
 * page lists them in by default.
 *
 * Einführung Brauereiwesen has a third lecture, `VL 3: Rückblick`, which is neither
 * recorded nor live and so belongs in none of the page's sections.
 */
export const recordings: Record<CourseKey, string[]> = {
  brauereiwesen: ["VL 2: Wie mache ich Bier?", "VL 1: Was ist Bier?"],
  games101: [],
  godev: ["VL 1: Intro to Go"],
  bierkunde: ["VL 1: Reinheitsgebot"],
  geheim: [],
};

/**
 * A recorded lecture of Einführung Brauereiwesen that is marked private. Only the
 * people who administer that course are sent it at all, and they are shown it marked
 * as withheld from everyone else.
 */
export const privateLecture = "VL 4: Interna";

/** A lecture that must never be rendered: scheduled for a date that has passed, and
 * never recorded, so it belongs in none of the page's sections. */
export const unlistedLecture = "VL 3: Rückblick";

/**
 * The time-dependent lectures, all in Einführung Brauereiwesen except the last.
 *
 * Their dates are relative to when the dump was loaded, because "today" cannot be
 * written as a fixed date. Reload with `make e2e_db` before a run.
 */
export const schedule = {
  /** Later today, so the start page has something under "Today". */
  today: "VL 5: Heute Abend",
  /** The hour of the evening it starts; after this it has begun and "Today" empties. */
  todayStartsAt: { hours: 23, minutes: 45 },
  /**
   * Four of them, which is one more than the course page shows before "Show all".
   * `today` is scheduled as well until the evening it starts, so which three are shown
   * first shifts during the day — the tests assert how many, not which.
   */
  planned: ["VL 6: Hefe", "VL 7: Malz", "VL 8: Hopfen", "VL 9: Wasser"],
  plannedShown: 3,
  /**
   * Starting within the half hour, which is what opens the waiting room. In the
   * enrolled-only course, so that it cannot appear under "Today" for the students who
   * are not in that course.
   */
  comingUp: { course: "bierkunde", lecture: "VL 2: Verkostung" },
} as const;

/** studi1's watch progress in Einführung Brauereiwesen. */
export const watched = {
  user: "studi1",
  course: "brauereiwesen",
  /** Watched in full: hidden by the "Hide watched" filter, marked with an eye. */
  finished: "VL 1: Was ist Bier?",
  /** Half watched: still listed, with a half-filled progress bar. */
  partial: "VL 2: Wie mache ich Bier?",
} as const;

/**
 * studi3 has pinned two courses, and may only see one of them: a pin outlives the
 * access that created it, so the pinned listing has to drop the other.
 */
export const pinned = {
  user: "studi3",
  visible: ["brauereiwesen"],
  /** Enrolled-only, and studi3 is not enrolled. */
  hiddenFromThem: "bierkunde",
} as const;

/** Both are active from before the dump was loaded until long after. */
export const serverNotifications = [
  { warn: false, text: "Am Wochenende finden Wartungsarbeiten statt." },
  { warn: true, text: "Livestreams können heute unterbrochen sein." },
] as const;

export const courseUrl = (slug: CourseKey): string => {
  const { year, term } = courses[slug];
  return `/course/${year}/${term}/${slug}`;
};

/**
 * What each role is told it may do, keyed by `model.User.Role`.
 *
 * A restatement of `rolePermissions` in model/permissions.go, as GET /users/me reports
 * it. Written out per role rather than derived, for the same reason the rest of this
 * file is: a test that computed the expectation the way the server does would agree
 * with the server however wrong both were.
 */
export const permissionsByRole: Record<number, string[]> = {
  1: [
    "server.administer",
    "courses.administer.all",
    "courses.view.all",
    "users.manage",
    "lecture",
  ],
  // A lecturer administers only the courses granted to them, which is not a
  // permission — see `administers` above.
  2: ["lecture"],
  4: [],
};

/**
 * The two runners, added to the dump for the administration page — runners register
 * themselves over gRPC, so a fixture has no other way to contain any.
 *
 * Both are dead and no fixture can do otherwise: model.Runner.Alive is a heartbeat
 * within the last five seconds, which nothing loaded ahead of a run can satisfy. The
 * live rendering is covered by the unit tests instead.
 *
 * `beta` is consumed by the delete test in runners.spec.ts and cannot be put back, so
 * reload with `make e2e_db` before a second run.
 */
export const runners = {
  alpha: { hostname: "runner-alpha", version: "1.4.2", jobCount: 2, draining: false },
  beta: { hostname: "runner-beta", version: "1.3.0", jobCount: 0, draining: true },
} as const;
