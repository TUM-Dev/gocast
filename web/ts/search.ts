import {Semester} from "./api/semesters";

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
    let url = new URL(document.location.href);
    let params = new URLSearchParams(url.search);
    return params.has("slug") && params.has("year") && params.has("term");

}

export function searchPlaceholder() {
    if (isInCourse()) {
        return "Search in course";
    }
    return "Search for course";
}

function getSemestersString(years: number[], teachingTerms: string[]) : string {
    let ret = "";
    if (years.length == 0 && teachingTerms.length == 0) {
        return ret;
    } else if (years.length == teachingTerms.length) {
        for (let i = 0; i < years.length; i++) {
            if(i == years.length - 1) {
                ret += years[i] + teachingTerms[i];
            } else {
                ret += years[i] + teachingTerms[i] + ",";
            }
        }
    }
    return ret;
}

export function filteredSearch() {
    return {
        hits: {},
        open: false,
        searchInput: "",
        search: function (years: number[], teachingTerms: string[], courses: string[], limit: number = 10) {
            if (this.searchInput.length > 2) {
                if (years.length < 8 && teachingTerms.length < 8 && teachingTerms.length == years.length && courses.length < 2) {
                    fetch(`/api/search?q=${this.searchInput}${years.length > 0 ? encodeURIComponent('&semester=' + getSemestersString(years, teachingTerms)) : ""}${courses.length > 0 ? encodeURIComponent('&courses=' + courses.join(",")) : ""}&limit=${limit}`)
                        .then((res) => {
                                if (res.ok) {
                                    res.json().then((data) => {
                                        this.hits = data;
                                        this.open = true;
                                    });
                                }
                            }
                        );
                }
            } else {
                this.hits = {};
                this.open = false;
            }

        },
        searchWithDataFromPage: function (semesters: Semester[], selectedSemesters: number[]) {
            // TODO: Filter semesters with selected semesters
            let years = [];
            let teachingTerms = [];
            let courses = [];

            for (let i = 0; i < selectedSemesters.length; i++) {
                years.push(semesters[i].Year);
                teachingTerms.push(semesters[i].TeachingTerm);
            }
            this.search(years, teachingTerms, courses);
        }
    }
}

export function globalSearch() {
    return {
        hits: {},
        open: false,
        searchInput: "",
        search: function (year: number = -1, teachingTerm: string = "", limit: number = 10) {
            if (this.searchInput.length > 2) {
                let url = new URL(document.location.href);
                let params = new URLSearchParams(url.search);
                if (params.has("slug") && params.has("year") && params.has("term")) {
                    fetch(`/api/search?q=${this.searchInput}&course=${params.get("slug")}${params.get("year")}${params.get("term")}`).then((res) => {
                        if (res.ok) {
                            res.json().then((data) => {
                                this.hits = data;
                                this.open = true;
                            });
                        }
                    });
                }
                else if(year != -1 && teachingTerm != "") {
                    fetch(`/api/search?q=${this.searchInput}&limit=${limit}&semester=${year}${teachingTerm}`).then((res) => {
                        if (res.ok) {
                            res.json().then((data) => {
                                this.hits = data;
                                this.open = true;
                            });
                        }
                    });
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
    console.log("Initialized popstate listener")
    document.body.addEventListener("click", (event) => {
        setTimeout(() => {}, 50);
        (document.getElementById("search-courses") as HTMLInputElement).placeholder = searchPlaceholder();
    })
}