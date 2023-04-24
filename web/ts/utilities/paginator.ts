export class Paginator<T> {
    private list: T[];
    private split_number: number;

    private index: number;

    constructor(list: T[], split_number: number) {
        this.list = list;
        this.split_number = split_number;
        this.index = 1;
    }

    get(): T[] {
        return this.list.slice(0, this.index * this.split_number);
    }

    set(list: T[]) {
        this.list = list;
    }

    next() {
        this.index++;
    }

    hasNext() {
        return (
            Math.ceil(this.list.length / this.split_number) * this.split_number >= (this.index + 1) * this.split_number
        );
    }

    forEach(callback: (obj: T, i: number) => void) {
        this.list.forEach(callback);
    }

    hasElements() {
        return this.list.length > 0;
    }

    reset() {
        this.index = 1;
    }
}
