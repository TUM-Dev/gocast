export class SmartArray<T> {
    protected list: T[];

    constructor(list: T[]) {
        this.list = list;
    }

    get(sortFn?: CompareFunction<T>, filterPred?: FilterPredicate<T>): T[] {
        const copy = filterPred ? [...this.list].filter(filterPred) : [...this.list];
        return sortFn ? copy.sort(sortFn) : copy;
    }

    set(list: T[]): SmartArray<T> {
        this.list = list;
        return this;
    }

    forEach(callback: (obj: T, i: number) => void): SmartArray<T> {
        this.list.forEach(callback);
        return this;
    }

    hasElements() {
        return this.list.length > 0;
    }
}

export class GroupedSmartArray<T, K extends keyof never> {
    private list: T[];
    private key: (i: T) => K;

    constructor(list: T[], key: (i: T) => K) {
        this.list = list;
        this.key = key;
    }

    get(sortFn?: CompareFunction<T>, filterPred?: FilterPredicate<T>) {
        const copy = filterPred ? [...this.list].filter(filterPred) : [...this.list];
        const _list = sortFn ? copy.sort(sortFn) : copy;
        return groupBy(_list, this.key);
    }

    set(list: T[], key: (i: T) => K): GroupedSmartArray<T, K> {
        this.list = list;
        this.key = key;
        return this;
    }

    hasElements() {
        return this.list.length > 0;
    }
}

export type CompareFunction<T> = (a: T, b: T) => number;
export type FilterPredicate<T> = (o: T) => boolean;

const groupBy = <T, K extends keyof never>(arr: T[], key: (i: T) => K) =>
    arr.reduce((groups, item) => {
        (groups[key(item)] ||= []).push(item);
        return groups;
    }, {} as Record<K, T[]>);
