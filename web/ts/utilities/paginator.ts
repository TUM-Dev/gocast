export class Paginator<T> {
    private list: T[];
    private split_number: number;

    private index: number;

    constructor(list: T[], split_number: number) {
        this.list = list;
        this.split_number = split_number;
        this.index = 1;
    }

    get(sortFn?: (a: T, b: T) => number, filterPred?: (o: T) => boolean): T[] {
        const copy = filterPred ? [...this.list].filter(filterPred) : [...this.list];
        return sortFn
            ? copy.sort(sortFn).slice(0, this.index * this.split_number)
            : copy.slice(0, this.index * this.split_number);
    }

    set(list: T[]) {
        this.list = list;
    }

    next(all = false) {
        this.index = all ? this.list.length / this.split_number : this.index + 1;
    }

    hasNext() {
        return Math.ceil(this.list.length / this.split_number) >= this.index + 1;
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
