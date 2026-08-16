/**
 * Test environment setup.
 *
 * Node exposes a built-in `localStorage` global that is inert unless the process is
 * started with --localstorage-file, and it shadows the one the DOM environment would
 * otherwise provide. Modules that persist state — notifications, the theme — would
 * then fail for a reason that has nothing to do with them, so install a working
 * in-memory implementation instead.
 */

class MemoryStorage implements Storage {
  private store = new Map<string, string>();

  get length(): number {
    return this.store.size;
  }

  clear(): void {
    this.store.clear();
  }

  getItem(key: string): string | null {
    return this.store.get(key) ?? null;
  }

  key(index: number): string | null {
    return [...this.store.keys()][index] ?? null;
  }

  removeItem(key: string): void {
    this.store.delete(key);
  }

  setItem(key: string, value: string): void {
    this.store.set(key, String(value));
  }
}

Object.defineProperty(globalThis, "localStorage", {
  value: new MemoryStorage(),
  writable: true,
  configurable: true,
});
