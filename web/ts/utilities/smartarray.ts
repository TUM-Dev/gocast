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

export class GroupedSmartArray<T, K> {
    private list: T[];
    private key: (i: T) => K;

    constructor() {
        this.list = [];
    }

    get(sortFn?: CompareFunction<T>, filterPred?: FilterPredicate<T>) {
        const copy = filterPred ? [...this.list].filter(filterPred) : [...this.list];
        const _list = sortFn ? copy.sort(sortFn) : copy;
        return this.group(_list, this.key);
    }

    set(list: T[], key: (i: T) => K): GroupedSmartArray<T, K> {
        this.list = list;
        this.key = key;
        return this;
    }

    hasElements() {
        return this.list.length > 0;
    }

    private group(list: T[], key: (i: T) => K) {
        const groups = [];

        let lastKey = null;
        let currentGroup = [];

        list.forEach((l) => {
            if (lastKey !== null && key(l) != lastKey) {
                groups.push(currentGroup);
                currentGroup = [];
            }
            currentGroup.push(l);
            lastKey = key(l);
        });
        groups.push(currentGroup);
        return groups;
    }
}

export type CompareFunction<T> = (a: T, b: T) => number;
export type FilterPredicate<T> = (o: T) => boolean;
