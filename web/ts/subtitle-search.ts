export function subtitleSearch(streamID: number) {
    return {
        hits: [],
        open: false,
        lastEventTimestamp: 0,
        search: function (query: string) {
            if (query.length > 2) {
                fetch(`/api/search/stream/${streamID}/subtitles?q=${query}`).then((res) => {
                    if (res.ok) {
                        res.json().then((data) => {
                            this.hits = data.hits;
                            this.open = this.hits.length > 0;
                        });
                    }
                });
            } else {
                this.hits = [];
                this.open = false;
            }
        },
        openRes: function () {
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
        },
    };
}
