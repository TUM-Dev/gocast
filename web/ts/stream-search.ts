export class StreamSearch {
    readonly courseId: number;

    timeout: NodeJS.Timeout;
    loading: boolean;
    currentQ: string;
    focusIndex: number;

    results: searchResponseItem[];

    constructor(courseId: number) {
        this.courseId = courseId;
        this.reset();
    }

    reset() {
        this.results = [];
        this.loading = false;
        this.currentQ = "";
        this.focusIndex = 0;
    }

    async performSearch() {
        clearTimeout(this.timeout);

        this.loading = true;
        this.timeout = setTimeout(async () => {
            if (this.currentQ !== "") {
                await fetch(`/api/search/streams?q=${this.currentQ}&courseId=${this.courseId}`)
                    .then((res) => {
                        if (res.ok) {
                            return res.json();
                        }
                        throw new Error("Can not perform search");
                    })
                    .then((sr: searchResponse) => {
                        this.focusIndex = 0;
                        this.results = sr.results;
                        this.loading = false;

                        document.dispatchEvent(new CustomEvent("showresults"));
                    })
                    .catch((error) => {
                        console.log(error);
                        this.reset();
                    });
            } else {
                this.reset();
            }
        }, 250);
    }

    OnBackspace(e: KeyboardEvent) {
        if (e.code === "Backspace") {
            this.performSearch();
        }
    }

    focusUp() {
        this.focusIndex = this.focusIndex === 0 ? this.results.length - 1 : this.focusIndex - 1;
    }

    focusDown() {
        this.focusIndex = (this.focusIndex + 1) % this.results.length;
    }
}

type searchResponse = {
    duration: number;
    results: searchResponseItem[];
};

type searchResponseItem = {
    friendlyTime: string;
    name: string;
};

export class SearchContext {
    show: boolean;

    constructor() {
        this.show = false;
    }

    showModal() {
        this.show = true;
        setTimeout(() => {
            document.getElementById("search-input").focus();
        }, 250); // execute after alpine re-render
    }

    hideModal(streamSearch: StreamSearch) {
        this.show = false;
        streamSearch.reset();
    }
}
