import {throttle, ThrottleFunc} from "./throttle";

export enum LogLevel {
    none,
    warn,
    info,
    debug,
}

const logLevelNames: string[] = ["None", "Warn", "Info", "Debug"];

export interface DirtyState {
    isDirty: boolean;
    dirtyKeys: string[];
}

export interface ChangeSetOptions<T> {
    comparator?: (key: string, a: T, b: T) => boolean,
    updateTransformer?: ComputedProperties<T>,
    onUpdate?: (changeState: T, dirtyState: DirtyState) => void,
    updateThrottle?: number,
    logLevel?: LogLevel,
};

/**
 * ## ChangeSet Class
 *
 * The `ChangeSet` class is designed to manage and track changes to a state object.
 * It provides an encapsulated way of observing state changes, committing them,
 * or rolling them back. Essentially, it helps in maintaining two versions of a state:
 * one that represents the current, committed state and another that captures all the changes (dirty state).
 *
 * ### Features
 * - **DirtyState**: Utilizes a `DirtyState` object to indicate if the state is dirty (modified but not committed)
 *   and which keys in the state object are dirty.
 * - **Custom Comparators**: Optionally, you can pass in a custom comparator function to determine how to compare state objects.
 * - **Event Subscriptions**: Offers an API for listening to changes in the state.
 *
 * ### Example Usage
 * ```typescript
 * const myState = { key1: 'value1', key2: 'value2' };
 * const changeSet = new ChangeSet(myState);
 *
 * changeSet.listen((changeState, dirtyState) => {
 *   console.log("Changed State:", changeState);
 *   console.log("Is Dirty:", dirtyState.isDirty);
 * });
 *
 * changeSet.patch('key1', 'new_value1');
 * ```
 *
 * ### Methods
 * - `listen(onUpdate)`: Subscribe to state changes.
 * - `removeListener(onUpdate)`: Unsubscribe from state changes.
 * - `get()`: Get the current uncommitted state.
 * - `set(val)`: Set the change state.
 * - `patch(key, val, options)`: Patch a specific key in the state object.
 * - `updateState(state)`: Update the state object, and reconcile it with the change state.
 * - `commit(options)`: Commit the change state, making it the new committed state.
 * - `reset()`: Reset the change state back to the last committed state.
 * - `isDirty()`: Check if the state has changes that are not yet committed.
 * - `changedKeys()`: Get the keys that have uncommitted changes.
 *
 * ### Alpine.js `bind-change-set` Directive Example
 * The `ChangeSet` class seamlessly integrates with Alpine.js through the `bind-change-set` directive.
 * For example, to bind a text input element to a `ChangeSet` instance, you can use the following HTML snippet:
 *
 * ```html
 * <input type="text" name="firstName" x-bind-change-set="changeSet">
 * ```
 *
 * This makes it a useful utility for handling state changes in a predictable way.
 */
export class ChangeSet<T> {
    private state: T;
    private changeState: T;
    private readonly comparator?: PropertyComparator<T>;
    private onUpdate: ((changeState: T, dirtyState: DirtyState) => void)[];
    private readonly changeStateTransformer?: ((changeState: T) => T);
    private readonly stateTransformer?: ((changeState: T) => T);

    private readonly throttledDispatchUpdateNoStateChanged?: ThrottleFunc;
    private readonly throttledDispatchUpdateStateChanged?: ThrottleFunc;

    private readonly logLevel: LogLevel;
    private lastLogTimestamp: number;
    private static logIdCounter: number = 0;
    private readonly logId: number;

    constructor(
        state: T,
        { comparator, updateTransformer, onUpdate, updateThrottle = 50, logLevel = LogLevel.none }: ChangeSetOptions<T> = {}
    ) {
        this.lastLogTimestamp = Date.now();
        this.logLevel = logLevel;
        this.logId = ChangeSet.logIdCounter++;
        this.state = state;
        this.onUpdate = onUpdate ? [onUpdate] : [];
        this.changeStateTransformer = updateTransformer !== undefined ? updateTransformer.create() : undefined;
        this.stateTransformer = updateTransformer !== undefined ? updateTransformer.create() : undefined;
        this.comparator = comparator;
        this.throttledDispatchUpdateNoStateChanged = throttle(() => this._dispatchUpdate(false), updateThrottle, true);
        this.throttledDispatchUpdateStateChanged = throttle(() => this._dispatchUpdate(true), updateThrottle, true);
        this.init();
    }

    /**
     * Add listener to receive change set updates
     * @param onUpdate
     */
    listen(onUpdate: (changeState: T, dirtyState: DirtyState) => void) {
        this._log(LogLevel.debug, "Add Update Listener", { onUpdate });
        this.onUpdate.push(onUpdate);
    }

    /**
     * Remove listener from change set.
     * @param onUpdate
     */
    removeListener(onUpdate: (changeState: T, dirtyState: DirtyState) => void) {
        this._log(LogLevel.debug, "Remove Update Listener", { onUpdate });
        this.onUpdate = this.onUpdate.filter((o) => o !== onUpdate);
    }

    /**
     * Returns a key from the change-state or the last committed state if flag is set
     * @param key key to return
     * @param lastCommittedState if set to true, value of the last committed state is returned
     */
    getValue(key: string, { lastCommittedState = false } = {}): T {
        if (lastCommittedState) {
            return this.state[key];
        }
        return this.changeState[key];
    }

    /**
     * Returns the current uncommitted change state.
     */
    get(): T {
        return this.changeState;
    }

    /**
     * Sets the change state.
     * @param val
     */
    set(val: T) {
        this._log(LogLevel.info, "Set ChangeState", { val });
        this.changeState = { ...val };
        this.dispatchUpdateThrottled(false);
    }

    /**
     * Patches a key with a new value. This makes the state dirty.
     * @param key
     * @param val
     * @param isCommitted if true, the data will be passed also to the state, and won't make the model dirty.
     */
    /* eslint-disable @typescript-eslint/no-explicit-any */
    patch(key: string, val: any, { isCommitted = false }: { isCommitted?: boolean } = {}) {
        this._log(LogLevel.info, "Patch State", { key, val, isCommitted });
        this.changeState = { ...this.changeState, [key]: val };
        if (isCommitted) {
            this.state = { ...this.state, [key]: val };
        }
        this.dispatchUpdateThrottled(isCommitted);
    }

    /**
     * Updates the state. Also updates all keys that are not dirty on the change-state, so they remain "undirty".
     * @param state
     */
    updateState(state: T) {
        this._log(LogLevel.info, "Update State", { state });
        const changedKeys = this.changedKeys();
        this.state = { ...state };

        for (const key of Object.keys(this.state)) {
            if (!changedKeys.includes(key)) {
                this.changeState[key] = this.state[key];
            }
        }
        this.dispatchUpdateThrottled(true);
    }

    /**
     * Commits the change state to the state. State is updated to the latest change state afterwards.
     * @param discardKeys List of keys that should be discarded and not committed.
     */
    commit({ discardKeys = [] }: { discardKeys?: string[] } = {}): void {
        this._log(LogLevel.info, "Perform Commit", { discardKeys });
        for (const key in discardKeys) {
            this.changeState[key] = this.state[key];
        }
        this.state = { ...this.changeState };
        this.dispatchUpdateThrottled(true);
    }

    /**
     * Init new state
     */
    init(): void {
        this._log(LogLevel.info, "Perform Init");
        this.changeState = { ...this.state };
        this.dispatchUpdateThrottled(true);
    }

    /**
     * Resets the change state to the state. Change state is the most current state afterwards.
     */
    reset(): void {
        this._log(LogLevel.info, "Perform Reset");
        this.changeState = { ...this.state };
        this.dispatchUpdateThrottled(false);
    }

    /**
     * A flag that indicated whether the change state is the same then the state or not.
     */
    isDirty(): boolean {
        for (const key of Object.keys(this.state)) {
            if (this.keyChanged(key)) {
                return true;
            }
        }
        return false;
    }

    /**
     * Returns the keys that are not the same between the state and the change state.
     */
    changedKeys(): string[] {
        const res = [];
        for (const key of Object.keys(this.state)) {
            if (this.keyChanged(key)) {
                res.push(key);
            }
        }
        return res;
    }

    /**
     * Checks if a specific key's value is different on the state and the change state.
     * @param key Key that should be checked
     */
    keyChanged(key: string): boolean {
        // Check with custom comparator if set
        if (this.comparator !== undefined) {
            const result = this.comparator(key, this.state, this.changeState);
            if (result !== null) {
                return result;
            }
        }

        // else just check if equiv
        return this.state[key] !== this.changeState[key];
    }

    /**
     * Executes all onUpdate listeners
     * @param stateChanged if state changed, state computed values are recalculated
     */
    _dispatchUpdate(stateChanged: boolean) {
        this._log(LogLevel.info, "Dispatch Update", { stateChanged });

        if (stateChanged && this.stateTransformer) {
            this._log(LogLevel.info, "Reevaluate Computed on State");
            this.state = this.stateTransformer(this.state);
        }

        if (this.changeStateTransformer) {
            this._log(LogLevel.info, "Reevaluate Computed on ChangeState");
            this.changeState = this.changeStateTransformer(this.changeState);
        }

        if (this.onUpdate.length > 0) {
            this._log(LogLevel.info, `Trigger ${this.onUpdate.length} onUpdate handler`);
            const dirtyKeys = this.changedKeys();
            for (const onUpdate of this.onUpdate) {
                onUpdate(this.changeState, {
                    dirtyKeys,
                    isDirty: dirtyKeys.length > 0,
                });
            }
        }
    }

    /**
     * Log Function that prints messages to console or discards them if the loglevel of changeSet is lower than the
     * loglevel of the message
     * @param level LogLevel of the message, if lower than changeSet's loglevel it will be discarded
     * @param message The message to print to console
     * @param payload Any payload object, "instance" is automatically attached if the changeSet loglevel is set to "debug"
     */
    _log(level: LogLevel, message: string, payload?: object) {
        if (level <= this.logLevel) {
            const ts = Date.now();
            const tsDelta = ts - this.lastLogTimestamp;
            this.lastLogTimestamp = ts;

            if (this.logLevel === LogLevel.debug) {
                payload = { ...(payload ?? {}), instance: this };
            }

            console.log(`[CHANGESET | ${this.logId.toString().padStart(4, "0")} | ${logLevelNames[level].padEnd(5, " ")}] ${message}`, payload ?? "", `[${tsDelta}ms]`);
        }
    }

    /**
     * Executes all onUpdate listeners
     * @param stateChanged if state changed, state computed values are recalculated
     */
    dispatchUpdateThrottled(stateChanged: boolean) {
        this._log(LogLevel.debug, "Throttled: Dispatch Update", { stateChanged });
        if (stateChanged && this.stateTransformer) {
            return this.throttledDispatchUpdateStateChanged();
        } else {
            return this.throttledDispatchUpdateNoStateChanged();
        }
    }
}

export type PropertyComparator<T> = (key: string, a: T, b: T) => boolean | null;
export type SinglePropertyComparator<T> = (a: T, b: T) => boolean | null;

export function ignoreKeys<T>(list: string[]): PropertyComparator<T> {
    return (key: string, a, b) => {
        if (list.includes(key)) {
            return false;
        }
        return null;
    };
}

export function singleProperty<T>(key: string, comparator: SinglePropertyComparator<T>): PropertyComparator<T> {
    return (_key: string, a, b) => {
        if (_key !== key) {
            return null;
        }
        return comparator(a, b);
    };
}

export function multiProperty<T>(keys: string[], comparator: PropertyComparator<T>): PropertyComparator<T> {
    return (_key: string, a, b) => {
        if (!keys.includes(_key)) {
            return null;
        }
        return comparator(_key, a, b);
    };
}

export function comparatorPipeline<T>(list: PropertyComparator<T>[]): PropertyComparator<T> {
    return (key: string, a, b) => {
        for (const comparator of list) {
            const res = comparator(key, a, b);
            if (res === true) {
                return true;
            } else if (res === false) {
                return false;
            }
        }
        return null;
    };
}

export type ComputedPropertyTransformer<T> = ((state: T) => T);
export type ComputedPropertySubTransformer<T> = {
    transform: ((state: T, oldState: T) => T);
    key: string;
};

export class ComputedProperties<T> {
    private readonly computed: ComputedPropertySubTransformer<T>[];

    constructor(computed: ComputedPropertySubTransformer<T>[]) {
        this.computed = computed;
    }

    create(): ComputedPropertyTransformer<T> {
        let oldState: T|null = null;
        return (state: T) => {
            for (const transformer of this.computed) {
                state = transformer.transform(state, oldState);
            }
            oldState = {...state};
            return state;
        };
    }

    /**
     * This is syntactic sugar to quickly ignore the computed keys in a changeset
     */
    ignore(): PropertyComparator<T> {
        return ignoreKeys(this.computed.map((transformer) => transformer.key));
    }
}

export function computedProperty<T, R>(key: string, updater: (changeState: T, old: T|null) => R, deps: string[] = []): ComputedPropertySubTransformer<T> {
    return {
        transform: (state: T, oldState: T|null) => {
            if (oldState == null || deps.length == 0 || deps.some((k) => oldState[k] !== state[k])) {
                state[key] = updater(state, oldState);
            }
            return state;
        },
        key,
    };
}