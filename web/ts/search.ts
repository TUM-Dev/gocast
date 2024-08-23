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

export function globalSearch() {
    return {
        hits: {},
        open: false,
        searchInput: "",
        search: function (year: number, teachingTerm: string) {
            if (this.searchInput.length > 2) {
                fetch(`/api/search?q=${this.searchInput}`).then((res) => {
                    if (res.ok) {
                        res.json().then((data) => {
                            this.hits = data;
                            this.open = true;
                            console.log(this.hits)
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