enum SearchDomain {
    VideoSections = "sections",
    Streams = "streams",
    Chats = "chats",
}

export class StreamSearch {
    readonly courseId: number;

    timeout: NodeJS.Timeout;
    loading: boolean;

    results: object[];

    constructor(courseId: number, slug: string) {
        this.courseId = courseId;
        this.results = [];
        this.loading = false;
    }

    getResults(results: number[][]): object[] {
        // flatten array of array, preserving only the first occurrence of each streamId
        const all = [];

        results.forEach((result) => {
            result.forEach((streamId) => {
                if (!all.includes(streamId)) {
                    all.push(streamId);
                }
            });
        });
        return all;
    }

    reset() {
        this.results = [];
        this.loading = false;
    }

    async getStreamFromResult(streamId): Promise<object> {
        return await fetch("/api/stream/" + streamId)
            .then((res) => res.json())
            .catch((err) => console.log(err))
            .then((stream) => stream);
    }

    async performSearch(q: string) {
        if (q !== "") {
            clearTimeout(this.timeout);

            this.loading = true;
            this.timeout = setTimeout(() => {
                Promise.all([
                    this.search(q, SearchDomain.VideoSections),
                    this.search(q, SearchDomain.Streams),
                    this.search(q, SearchDomain.Chats),
                ]).then((searchResponses) => {
                    this.loading = false;
                    this.results = this.getResults(searchResponses.map((sr) => sr.StreamIds));
                });
            }, 250);
        } else {
            this.results = [];
            this.loading = false;
        }
    }

    private async search(q: string, searchDomain: SearchDomain): Promise<searchResponse> {
        return await fetch(`/api/search/streams/${searchDomain}?q=${q}&courseId=${this.courseId}`)
            .then((res) => res.json())
            .catch((err) => console.log(err))
            .then((sr: searchResponse) => sr);
    }
}

type searchResponse = {
    StreamIds: number[];
};
