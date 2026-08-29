import { computed, type ComputedRef } from "vue";
import { useRoute, type LocationQuery, type RouteParams } from "vue-router";

/**
 * Reads a query parameter that is expected once.
 *
 * A repeated parameter — `?year=2025&year=2026` — arrives as an array, and a bare
 * `?year` as null. Both are malformed rather than meaningful, so both read as absent
 * instead of being coerced into a value the caller would act on.
 */
export function singleQueryParam(query: LocationQuery, name: string): string | undefined {
  const value = query[name];
  return typeof value === "string" && value !== "" ? value : undefined;
}

/**
 * Reads a path parameter that is expected once, with the same rules as
 * singleQueryParam. Vue Router types a repeatable segment as an array.
 */
export function singleRouteParam(params: RouteParams, name: string): string | undefined {
  const value = params[name];
  return typeof value === "string" && value !== "" ? value : undefined;
}

/** The semester a page is about, or undefined for whichever one is current. */
export interface SemesterQuery {
  year: ComputedRef<number | undefined>;
  term: ComputedRef<"W" | "S" | undefined>;
}

/**
 * Reads the semester from `?year=&term=`, or from the path on the course route,
 * which carries it as `/course/:year/:term/:slug`. The sidebar has to highlight the
 * right semester on both, and reading only the query would show a course from an old
 * semester beside the current semester's listings.
 *
 * Absent means the current semester, which only the server knows: the course
 * endpoints fall back to it when asked for year 0, so absent is passed on as absent
 * rather than resolved here. Anything malformed is treated the same way — the old
 * page answered a bad term with a 404, but a URL is not worth an error page when
 * showing the current semester is what the visitor wanted.
 */
export function useSemesterQuery(): SemesterQuery {
  const route = useRoute();

  // The query wins where a route somehow has both; nothing registers such a route.
  const raw = (name: string): string | undefined =>
    singleQueryParam(route.query, name) ?? singleRouteParam(route.params ?? {}, name);

  const year = computed(() => {
    const raw_ = raw("year");
    if (raw_ === undefined) return undefined;
    const parsed = Number(raw_);
    return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
  });

  const term = computed(() => {
    const raw_ = raw("term");
    return raw_ === "W" || raw_ === "S" ? raw_ : undefined;
  });

  return { year, term };
}
