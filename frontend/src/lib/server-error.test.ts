import { expect, it } from "vitest";

import { readServerError } from "./server-error";

function documentWith(head: string): Document {
  return new DOMParser().parseFromString(`<html><head>${head}</head><body></body></html>`, "text/html");
}

it("reads the status and message Go put in the shell", () => {
  const doc = documentWith(
    '<meta name="gocast-error-status" content="403"><meta name="gocast-error-message" content="No entry &quot;here&quot;">',
  );
  expect(readServerError(doc)).toEqual({ status: 403, message: 'No entry "here"' });
});

it("reports nothing on a normal load", () => {
  expect(readServerError(documentWith(""))).toBeNull();
});

it("ignores a status that is not an error", () => {
  expect(readServerError(documentWith('<meta name="gocast-error-status" content="abc">'))).toBeNull();
});
