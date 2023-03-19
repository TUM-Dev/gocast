export class ValueStreamMap<T> {
    protected streams: Map<string, ValueStream<T>> = new Map<string, ValueStream<T>>();

    private getStream(groupKey: string): ValueStream<T> {
        if (this.streams[groupKey] == null) {
            this.streams[groupKey] = new ValueStream<T>();
        }
        return this.streams[groupKey];
    }

    subscribe(groupKey: string, listener: ValueListener<T>) {
        this.getStream(groupKey).subscribe(listener);
    }

    unsubscribe(groupKey: string, listener: ValueListener<T>) {
        this.getStream(groupKey).unsubscribe(listener);
    }

    add(groupKey: string, data: T) {
        this.getStream(groupKey).add(data);
    }
}

export class ValueStream<T> {
    protected listeners: ValueListener<T>[] = [];

    subscribe(listener: ValueListener<T>) {
        this.listeners.push(listener);
    }

    unsubscribe(listener: ValueListener<T>) {
        this.listeners = this.listeners.filter((l) => l !== listener);
    }

    add(data: T) {
        for (const listener of this.listeners) {
            listener(data);
        }
    }
}

export type ValueListener<T> = (value: T) => void;
