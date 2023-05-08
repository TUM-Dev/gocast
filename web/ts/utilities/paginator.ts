export class Paginator<T> {
    private list: T[];
    private split_number: number;

    private index: number;

    constructor(list: T[], split_number: number) {
        this.list = list;
        this.split_number = split_number;
        this.index = 1;
    }

    get(compareFn?: (a: T, b: T) => number): T[] {
        return compareFn
            ? [...this.list].sort(compareFn).slice(0, this.index * this.split_number)
            : [...this.list].slice(0, this.index * this.split_number);
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
