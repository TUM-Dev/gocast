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
        /*openRes: function () {
            if (this.lastEventTimestamp + 1000 < Date.now()) {
                this.lastEventTimestamp = Date.now();
                this.open = true;
            }
        },
        closeRes: function () {
            if (this.lastEventTimestamp + 1000 < Date.now()) {
                this.lastEventTimestamp = Date.now();
                this.open = false;
            }
        },*/
    };
}

export function isInCourse() {
    let url = new URL(document.location.href);
    let params = new URLSearchParams(url.search);
    if (params.has("slug") && params.has("year") && params.has("term")) {
        return true;
    }
    return false;
}

export function searchPlaceholder() {
    if (isInCourse()) {
        return "Search in course";
    }
    return "Search for course";
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