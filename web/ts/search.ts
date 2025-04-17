import { Semester } from "./api/semesters";
import { Course, CoursesAPI } from "./api/courses";
import { Alpine } from "alpinejs";

export function coursesSearch() {
    return {
        hits: [],
        open: false,
        searchInput: "",
        search: function (year: number, teachingTerm: string) {
            if (this.searchInput.length > 2) {
                fetch(`/api/search/courses?q=${this.searchInput}&semester=${year}${teachingTerm}`).then((res) => {
                    if (res.ok) {
                        res.json().then((data) => {
                            this.hits = data.results[0].hits;
                            this.open = true;
                        });
                    }
                });
            } else {
                this.hits = [];
                this.open = false;
            }
        },
    };
}

export function isInCourse() {
    const url = new URL(document.location.href);
    const params = new URLSearchParams(url.search);
    return params.has("slug") && params.has("year") && params.has("term");
}

export function searchPlaceholder() {
    if (isInCourse()) {
        return "Search in course";
    }
    return "Search for course";
}

function getSemestersString(years: number[], teachingTerms: string[]): string {
    let ret = "";
    if (years.length != teachingTerms.length) {
        return ret;
    }
    for (let i = 0; i < years.length; i++) {
        if (i == years.length - 1) {
            ret += years[i] + teachingTerms[i];
        } else {
            ret += years[i] + teachingTerms[i] + ",";
        }
    }
    return ret;
}

function getCoursesString(courses: Course[]): string {
    let ret = "";
    for (let i = 0; i < courses.length; i++) {
        if (i == courses.length - 1) {
            ret += courses[i].Slug + courses[i].Year + courses[i].TeachingTerm;
        } else {
            ret += courses[i].Slug + courses[i].Year + courses[i].TeachingTerm + ",";
        }
    }
    return ret;
}

export function filteredSearch() {
    return {
        hits: {},
        open: false,
        searchInput: "",
        search: function (years: number[], teachingTerms: string[], courses: Course[], limit: number = 20) {
            if (this.searchInput.length > 2) {
                if (
                    years.length < 8 &&
                    teachingTerms.length < 8 &&
                    teachingTerms.length == years.length &&
                    courses.length < 3
                ) {
                    fetch(
                        `/api/search?q=${this.searchInput}&semester=${encodeURIComponent(
                            getSemestersString(years, teachingTerms),
                        )}&course=${encodeURIComponent(getCoursesString(courses))}&limit=${limit}`,
                    ).then((res) => {
                        if (res.ok) {
                            res.json().then((data) => {
                                this.hits = data;
                                this.open = true;
                            });
                        }
                    });
                }
            } else {
                this.hits = {};
                this.open = false;
            }
        },
        searchWithDataFromPage: function (
            semesters: Semester[],
            selectedSemesters: number[],
            allCourses: Course[],
            selectedCourses: number[],
        ) {
            const years = [];
            const teachingTerms = [];
            const courses = [];

            for (let i = 0; i < selectedSemesters.length; i++) {
                years.push(semesters[selectedSemesters[i]].Year);
                teachingTerms.push(semesters[selectedSemesters[i]].TeachingTerm);
            }
            for (let i = 0; i < selectedCourses.length; i++) {
                courses.push(allCourses[selectedCourses[i]]);
            }
            this.search(years, teachingTerms, courses);
        },
    };
}

export function globalSearch() {
    return {
        hits: {},
        open: false,
        searchInput: "",
        search: function (year: number = -1, teachingTerm: string = "", limit: number = 10) {
            if (this.searchInput.length > 2) {
                const url = new URL(document.location.href);
                const params = new URLSearchParams(url.search);
                if (params.has("slug") && params.has("year") && params.has("term")) {
                    fetch(
                        `/api/search?q=${this.searchInput}&course=${params.get("slug")}${params.get(
                            "year",
                        )}${params.get("term")}`,
                    ).then((res) => {
                        if (res.ok) {
                            res.json().then((data) => {
                                this.hits = data;
                                this.open = true;
                            });
                        }
                    });
                } else if (year != -1 && teachingTerm != "") {
                    fetch(`/api/search?q=${this.searchInput}&limit=${limit}&semester=${year}${teachingTerm}`).then(
                        (res) => {
                            if (res.ok) {
                                res.json().then((data) => {
                                    this.hits = data;
                                    this.open = true;
                                });
                            }
                        },
                    );
                } else {
                    fetch(`/api/search?q=${this.searchInput}&limit=${limit}`).then((res) => {
                        if (res.ok) {
                            res.json().then((data) => {
                                this.hits = data;
                                this.open = true;
                            });
                        }
                    });
                }
            } else {
                this.hits = {};
                this.open = false;
            }
        },
    };
}

export function initPopstateSearchBarListener() {
    document.body.addEventListener("click", (event) => {
        setTimeout(() => {
            updateSearchBarPlaceholder();
        }, 50);
    });
    console.log("Initialized popstate listener");
}

export function updateSearchBarPlaceholder() {
    (document.getElementById("search-courses") as HTMLInputElement).placeholder = searchPlaceholder();
}

export function getSearchQueryFromParam() {
    const url = new URL(document.location.href);
    const params = new URLSearchParams(url.search);
    return params.get("q");
}

export function getCourseFromParam() {
    const url = new URL(document.location.href);
    const params = new URLSearchParams(url.search);
    return params.get("course");
}

export function getSemestersFromParam() {
    const url = new URL(document.location.href);
    const params = new URLSearchParams(url.search);
    return params.get("semester");
}

export function generateCourseFromParam() {
    const url = new URL(document.location.href);
    const params = new URLSearchParams(url.search);
    const slug = params.get("slug");
    const year = params.get("year");
    const term = params.get("term");
    return slug + year + term;
}

export function getYearFromCourse(course: string) {
    return parseInt(course.substring(course.length - 5, course.length - 1));
}

export function getTermFromCourse(course: string) {
    return course.substring(course.length - 1, course.length);
}

export function getSlugFromCourse(course: string) {
    return course.substring(0, course.length - 5);
}

export async function getCoursesOfSemesters(semesters: Semester[], filterSemesters: number[]): Promise<Course[]> {
    let courses: Course[] = [];
    for (let i = 0; i < filterSemesters.length; i++) {
        courses = courses.concat(
            await CoursesAPI.getPublic(semesters[filterSemesters[i]].Year, semesters[filterSemesters[i]].TeachingTerm),
        );
        courses = courses.concat(
            await CoursesAPI.getUsers(semesters[filterSemesters[i]].Year, semesters[filterSemesters[i]].TeachingTerm),
        );
    }
    courses = courses.filter((course, index, self) => self.findIndex((t) => t.Slug === course.Slug) === index);
    return [...new Set(courses)];
}

export function initSearchBarArrowKeysListener() {
    document.addEventListener("keydown", (event) => {
        if (document.getElementById("search-results") == null) {
            return;
        }
        const searchResults = document.getElementById("search-results").querySelectorAll("li[role='option']");
        const activeElement = document.activeElement as HTMLLIElement;
        if (event.key == "ArrowDown") {
            const currentIndex = Array.from(searchResults).indexOf(activeElement);
            const nextIndex = currentIndex + 1;
            if (nextIndex < searchResults.length) {
                (searchResults[nextIndex] as HTMLLIElement).focus();
            }
        } else if (event.key == "ArrowUp") {
            const currentIndex = Array.from(searchResults).indexOf(activeElement);
            const nextIndex = currentIndex - 1;
            if (nextIndex >= 0) {
                (searchResults[nextIndex] as HTMLLIElement).focus();
            }
        } else if (event.key == "Enter") {
            const currentIndex = Array.from(searchResults).indexOf(activeElement);
            if (currentIndex >= 0 && currentIndex < searchResults.length) {
                const curObj = searchResults[currentIndex];
                curObj.getElementsByTagName("a")[0].click();
            }
        }
    });
}
