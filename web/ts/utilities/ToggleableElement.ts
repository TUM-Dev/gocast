export class ToggleableElement {
    private readonly children: Map<string, ToggleableElement>;

    public value: boolean;

    constructor(children?: readonly [string, ToggleableElement][] | null, value = false) {
        this.children = children ? new Map<string, ToggleableElement>(children) : new Map<string, ToggleableElement>();
        this.value = value;
    }

    getChild(name: string): ToggleableElement {
        return this.children.get(name);
    }

    toggle(set?: boolean) {
        this.value = set ?? !this.value;
        if (!this.value) {
            this.children.forEach((c) => c.toggle(false));
        }
    }

    toggleText(a: string, b: string) {
        return this.value ? a : b;
    }
}
