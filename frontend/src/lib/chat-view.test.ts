import { describe, expect, it } from "vitest";

import type { ChatMessage, ChatPoll } from "./chat";
import {
  connectionNotice,
  countMessages,
  pollShare,
  reactionGlyph,
  type ConnectionState,
} from "./chat-view";

const open: ConnectionState = { status: "open", loaded: true };

describe("connectionNotice", () => {
  it("says nothing while the connection holds", () => {
    expect(connectionNotice(open)).toBeNull();
  });

  it("tells a first connection from a lost one", () => {
    expect(connectionNotice({ status: "connecting", loaded: false })?.text).toBe("Connecting…");
    expect(connectionNotice({ status: "connecting", loaded: true })?.text).toBe(
      "Connection lost. Reconnecting…",
    );
  });

  it.each([
    [4400, "not a valid chat address"],
    [4403, "not allowed"],
    [4404, "does not exist"],
  ])("explains the refusal %i as final", (fatalCode, text) => {
    const notice = connectionNotice({ status: "closed", loaded: true, fatalCode });
    expect(notice?.kind).toBe("error");
    expect(notice?.text).toContain(text);
  });

  it("offers sign-in when the server refused a visitor or expired a session", () => {
    expect(connectionNotice({ status: "closed", loaded: false, fatalCode: 4403 })?.signIn).toBe(true);
    const expired = connectionNotice({ status: "connecting", loaded: true, lastCloseCode: 4401 });
    expect(expired).toMatchObject({ kind: "error", signIn: true });
  });

  it("keeps saying the session expired once the connection has given up on it", () => {
    // After the second 4401 in a row the socket stops retrying: it is closed, not
    // connecting, and the fatal code is what is left to explain it.
    const notice = connectionNotice({ status: "closed", loaded: true, fatalCode: 4401 });
    expect(notice).toMatchObject({ kind: "error", signIn: true });
    expect(notice?.text).toContain("session has expired");
  });

  it("reports a failed snapshot", () => {
    expect(connectionNotice({ ...open, error: "boom" })?.text).toContain("boom");
  });
});

describe("chat helpers", () => {
  it("spells reactions, keeping an unknown name legible", () => {
    expect(reactionGlyph("tada")).toBe("\u{1F389}");
    expect(reactionGlyph("rocket")).toBe(":rocket:");
  });

  it("counts replies as messages", () => {
    const m = (replies: ChatMessage[] = []) => ({ replies }) as ChatMessage;
    expect(countMessages([m([m(), m()]), m()])).toBe(4);
    expect(countMessages([])).toBe(0);
  });

  it("shares votes out, and shows nothing before the first", () => {
    const poll: ChatPoll = {
      id: 1,
      question: "?",
      active: true,
      options: [
        { id: 1, answer: "a", votes: 3 },
        { id: 2, answer: "b", votes: 1 },
      ],
    };
    expect(pollShare(poll, 3)).toBe(75);
    expect(pollShare({ ...poll, options: [{ id: 1, answer: "a", votes: 0 }] }, 0)).toBe(0);
  });
});
