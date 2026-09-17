/**
 * Importing courses from TUMonline: search a department's room schedule for a date
 * range, let an administrator review and adjust the found courses, then create them.
 *
 * Two calls, mirroring the wizard's two steps — searching hits TUMonline directly and
 * can be slow or fail upstream, so it stays a separate, retryable step from creating
 * the courses.
 */

import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";

import {
  CourseImportRequestSchema,
  CourseImportResponseSchema,
  CourseImportSearchRequestSchema,
  CourseImportSearchResponseSchema,
  type CourseImportCourse,
  type CourseImportResult,
} from "@/gen/server/apiv2_pb";

import { apiPostMessage } from "./api";

/**
 * The TUMonline organisation unit ids for the departments this deployment has cameras
 * in, mirroring the constants in pkg/campus/campusonline (CsOrgId and friends). Not
 * exhaustive: "specify id" covers any other department by its numeric id directly, the
 * same escape hatch the legacy page offered.
 */
export const courseImportDepartments: { label: string; departmentId: number }[] = [
  { label: "Computer Science", departmentId: 53598 },
  { label: "Computer Engineering", departmentId: 53599 },
  { label: "Mathematics", departmentId: 53597 },
  { label: "Physics", departmentId: 53217 },
];

export interface CourseImportEventUI {
  start: Date;
  end: Date;
  roomName: string;
  comment: string;
  eventId: string;
  import: boolean;
}

export interface CourseImportContactUI {
  firstName: string;
  lastName: string;
  email: string;
  role: string;
  mainContact: boolean;
}

export interface CourseImportCourseUI {
  title: string;
  slug: string;
  courseId: number;
  language: string;
  import: boolean;
  events: CourseImportEventUI[];
  contacts: CourseImportContactUI[];
}

function fromProtoCourse(course: CourseImportCourse): CourseImportCourseUI {
  return {
    title: course.title,
    slug: course.slug,
    courseId: course.courseId,
    language: course.language,
    import: course.import,
    events: course.events.map((event) => ({
      start: event.start ? timestampDate(event.start) : new Date(0),
      end: event.end ? timestampDate(event.end) : new Date(0),
      roomName: event.roomName,
      comment: event.comment,
      eventId: event.eventId,
      import: event.import,
    })),
    contacts: course.contacts.map((contact) => ({
      firstName: contact.firstName,
      lastName: contact.lastName,
      email: contact.email,
      role: contact.role,
      mainContact: contact.mainContact,
    })),
  };
}

function toProtoCourse(course: CourseImportCourseUI): CourseImportCourse {
  return {
    $typeName: "protobuf.CourseImportCourse",
    title: course.title,
    slug: course.slug,
    courseId: course.courseId,
    language: course.language,
    import: course.import,
    events: course.events.map((event) => ({
      $typeName: "protobuf.CourseImportEvent",
      start: timestampFromDate(event.start),
      end: timestampFromDate(event.end),
      roomName: event.roomName,
      comment: event.comment,
      eventId: event.eventId,
      import: event.import,
    })),
    contacts: course.contacts.map((contact) => ({
      $typeName: "protobuf.CourseImportContact",
      firstName: contact.firstName,
      lastName: contact.lastName,
      email: contact.email,
      role: contact.role,
      mainContact: contact.mainContact,
    })),
  };
}

/**
 * Searches TUMonline's room schedule for the given department and date range, grouped
 * into courses with their contacts and language already resolved. Slow (TUMonline is
 * queried once per course found, to fetch contacts) and prone to upstream failures,
 * which are thrown as ApiError rather than swallowed — the legacy handler's `return`
 * with no response on this exact error is what left an administrator's request
 * hanging forever.
 */
export async function searchCourseImportSchedule(
  from: Date,
  to: Date,
  departmentId: number,
): Promise<CourseImportCourseUI[]> {
  const res = await apiPostMessage(
    CourseImportSearchRequestSchema,
    CourseImportSearchResponseSchema,
    "/admin/course-import/search",
    {
      $typeName: "protobuf.CourseImportSearchRequest",
      from: timestampFromDate(from),
      to: timestampFromDate(to),
      departmentId,
    },
  );
  return res.courses.map(fromProtoCourse);
}

export interface CourseImportResultUI {
  title: string;
  success: boolean;
  error: string;
}

function fromProtoResult(result: CourseImportResult): CourseImportResultUI {
  return { title: result.title, success: result.success, error: result.error };
}

/**
 * Creates a course and its streams for each course still marked `import`, and emails
 * its main contact an activation (or opt-out) link. One course failing -- a duplicate
 * slug, an LDAP lookup miss -- does not stop the rest; the result for each course is
 * reported back rather than folded into one all-or-nothing outcome.
 */
export async function importCourseImportCourses(
  year: number,
  term: "W" | "S",
  optIn: boolean,
  courses: CourseImportCourseUI[],
): Promise<CourseImportResultUI[]> {
  const res = await apiPostMessage(
    CourseImportRequestSchema,
    CourseImportResponseSchema,
    "/admin/course-import",
    {
      $typeName: "protobuf.CourseImportRequest",
      year,
      term,
      optIn,
      courses: courses.map(toProtoCourse),
    },
  );
  return res.results.map(fromProtoResult);
}
