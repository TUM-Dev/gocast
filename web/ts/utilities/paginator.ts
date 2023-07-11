export class Paginator<T> {
    protected list: T[];
    protected split_number: number;
    protected index: number;

    private readonly preloader: Preload<T>;

    constructor(list: T[], split_number: number, preloader?: Preload<T>) {
        this.list = list;
        this.split_number = split_number;
        this.index = 1;
        this.preloader = preloader;
    }

    get(sortFn?: CompareFunction<T>, filterPred?: FilterPredicate<T>): T[] {
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

    preload(sortFn?: CompareFunction<T>): Paginator<T> {
        const copy = [...this.list];
        if (this.hasNext() && this.preloader) {
            (sortFn ? copy.sort(sortFn) : copy)
                .sort(sortFn)
                .slice(this.index * this.split_number, (this.index + 1) * this.split_number)
                .forEach((el: T) => this.preloader(el));
        }
        return this;
    }
}

export class AutoPaginator<T> extends Paginator<T> {
    constructor(list: T[], split_number: number, preloader?: Preload<T>) {
        super(list, split_number, preloader);
    }

    registerAutoNextButton(el: HTMLElement) {
        const options = { root: document.getElementsByTagName("body")[0], rootMargin: "16px", threshold: 0.75 };
        const observer = new IntersectionObserver((_) => this.next(), options);
        observer.observe(el);
    }
}

type Preload<T> = (o: T) => void;

type CompareFunction<T> = (a: T, b: T) => number;
type FilterPredicate<T> = (o: T) => boolean;
