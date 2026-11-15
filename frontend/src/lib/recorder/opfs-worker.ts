/**
 * Writes a browser recording's chunks for OPFSChunkStore. A worker, because only a
 * worker gets a sync access handle, whose flush is what makes a write survive the
 * browser or the machine crashing. Writes are answered once flushed, one at a time, in
 * the order they were sent.
 */

interface WriteRequest {
  id: number;
  path: string[];
  data: ArrayBuffer;
}

interface SyncAccessHandle {
  truncate(size: number): void;
  write(buffer: ArrayBuffer, options?: { at?: number }): number;
  flush(): void;
  close(): void;
}

let queue = Promise.resolve();

async function write({ path, data }: WriteRequest): Promise<void> {
  let dir = await navigator.storage.getDirectory();
  for (const name of path.slice(0, -1)) {
    dir = await dir.getDirectoryHandle(name, { create: true });
  }
  const file = await dir.getFileHandle(path[path.length - 1], { create: true });
  const handle = (await (
    file as FileSystemFileHandle & { createSyncAccessHandle(): Promise<SyncAccessHandle> }
  ).createSyncAccessHandle()) as SyncAccessHandle;
  try {
    handle.truncate(0);
    handle.write(data, { at: 0 });
    handle.flush();
  } finally {
    handle.close();
  }
}

self.onmessage = (event: MessageEvent<WriteRequest>) => {
  const req = event.data;
  queue = queue.then(async () => {
    try {
      await write(req);
      self.postMessage({ id: req.id });
    } catch (err) {
      self.postMessage({ id: req.id, error: err instanceof Error ? err.message : String(err) });
    }
  });
};
