export class LectureSearch {
    readonly courseId: number;

    results: number[];

    constructor(courseId) {
        this.courseId = courseId;
    }

    async performSearch(q: string) {
        await this.searchInSections(q);
        await this.searchInStreams(q);
    }

    private async searchInSections(q: string) {
        return await fetch(`/api/search/sections?q=${q}&courseId=${this.courseId}`)
            .then((res) => res.json())
            .catch((err) => console.log(err))
            .then((sr: searchResponse) => {
                console.log(sr);
            });
    }

    private async searchInStreams(q: string) {
        return await fetch(`/api/search/streams?q=${q}&courseId=${this.courseId}`)
            .then((res) => res.json())
            .catch((err) => console.log(err))
            .then((sr: searchResponse) => {
                console.log(sr);
            });
    }
}

type searchResponse = {
    streamIds: number[];
};
