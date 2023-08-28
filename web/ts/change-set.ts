export class ChangeSet<T> {
    private state: T;
    private changeState: T;
    private readonly comparator?: (key: string, a: T, b: T) => boolean|null;

    constructor(state: T, comparator?: (key: string, a: T, b: T) => boolean) {
        this.state = state;
        this.comparator = comparator;
        this.reset();
    }

    get(): T {
        return this.changeState;
    }

    set(val: T) {
        this.changeState = {...val};
    }

    commit(): void {
        this.state = {...this.changeState};
    }

    reset(): void {
        this.changeState = {...this.state};
    }

    isDirty(): boolean {
        for (const key of Object.keys(this.state)) {
            if (this.keyChanged(key)) {
                return true;
            }
        }
        return false;
    }

    changedKeys(): string[] {
        const res = [];
        for (const key of Object.keys(this.state)) {
            if (this.keyChanged(key)) {
                res.push(key);
            }
        }
        return res;
    }

    keyChanged(key: string): boolean {
        // Check with custom comparator if set
        if (this.comparator !== undefined) {
            const result = this.comparator(key, this.state, this.changeState);
            if (result !== null) {
                return result;
            }
        }

        // Otherwise just check if equiv
        if (this.state[key] !== this.changeState[key]) {
            return true;
        }
        return false;
    }
}
