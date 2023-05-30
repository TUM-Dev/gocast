export class Paginator<T> {
    private list: T[];
    private split_number: number;

    private index: number;

    private readonly preloader: Preload<T>;

    constructor(list: T[], split_number: number, preloader?: Preload<T>) {
        this.list = list;
        this.split_number = split_number;
        this.index = 1;
        this.preloader = preloader;
    }

    get(sortFn?: (a: T, b: T) => number, filterPred?: (o: T) => boolean): T[] {
        const copy = filterPred ? [...this.list].filter(filterPred) : [...this.list];
        return sortFn
            ? copy.sort(sortFn).slice(0, this.index * this.split_number)
            : copy.slice(0, this.index * this.split_number);
    }

    set(list: T[]): Paginator<T> {
        this.list = list;
        return this;
    }

    next(all = false) {
        this.index = all ? this.list.length / this.split_number : this.index + 1;
        this.preload();
    }

    hasNext() {
        return Math.ceil(this.list.length / this.split_number) >= this.index + 1;
    }

    forEach(callback: (obj: T, i: number) => void): Paginator<T> {
        this.list.forEach(callback);
        return this;
    }

    hasElements() {
        return this.list.length > 0;
    }

    reset(): Paginator<T> {
        this.index = 1;
        return this;
    }

    private preload() {
        if (this.hasNext() && this.preloader) {
            this.list
                .slice((this.index - 1) * this.split_number, this.index * this.split_number)
                .forEach((el: T) => this.preloader(el));
        }
    }
}

type Preload<T> = (o: T) => void;
