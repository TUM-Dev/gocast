export class CustomURL {
    private url: URL;

    constructor(url: string, searchParams?: object) {
        this.url = new URL(url, window.location.origin);
        if (searchParams) {
            for (const [key, value] of Object.entries(searchParams)) {
                if (value !== undefined) {
                    this.url.searchParams.set(key, value);
                }
            }
        }
    }

    toString(): string {
        return this.url.toString();
    }
}
