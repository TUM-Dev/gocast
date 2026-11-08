// Stands in for vite-plugin-pwa's virtual module, which only exists under Vite with the
// plugin loaded (vitest.config.ts aliases it here). The tests never register a worker.
import type { RegisterSWOptions } from "vite-plugin-pwa/types";

export function registerSW(_options?: RegisterSWOptions): (reloadPage?: boolean) => Promise<void> {
  return async () => {};
}
