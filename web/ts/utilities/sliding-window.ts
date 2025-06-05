import { Paginator } from "./paginator";

export class SlidingWindow<T> extends Paginator<T> {
    constructor(list: T[], split_number: number) {
        super(list, split_number);
    }

    get(sortFn?: CompareFunction<T>): T[] {
        const copy = [...this.list].filter(this.filterPred.bind(this));
        return sortFn
            ? copy.sort(sortFn).slice(0, this.index * this.split_number)
            : copy.slice(0, this.index * this.split_number);
    }

    isInWindow(o: T): boolean {
        const i = this.list.findIndex((o1) => o1 === o);
        return i !== -1 && this.filterPred(o, i);
    }

    show(o: T) {
        const i = this.list.findIndex((o1) => o1 === o);
        this.index = Math.floor(i / this.split_number) + 1;
    }

    prev() {
        this.index--;
    }

    hasPrev() {
        return this.index > 1;
    }

    private filterPred(o: T, index: number): boolean {
        return index >= (this.index - 1) * this.split_number && index < this.index * this.split_number;
    }
}

type CompareFunction<T> = (a: T, b: T) => number;
