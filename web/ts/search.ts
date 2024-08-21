export function coursesSearch() {
    return {
        hits: [],
        open: false,
        searchInput: "",
        search: function (year: number, teachingTerm: string) {
            if (this.searchInput.length > 2) {
                fetch(`/api/search/courses?q=${this.searchInput}&year=${year}&term=${teachingTerm}`).then((res) => {
                    if (res.ok) {
                        res.json().then((data) => {
                            this.hits = data.hits;
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

export function globalSearch() {
    return {
        hits: [],
        open: false,
        searchInput: "",
        search: function (year: number, teachingTerm: string) {
            if (this.searchInput.length > 2) {
                fetch(`/api/search?q=${this.searchInput}&semester=${year}${teachingTerm}`).then((res) => {
                    if (res.ok) {
                        res.json().then((data) => {
                            this.hits = data.hits;
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