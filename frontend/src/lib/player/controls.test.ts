import { describe, expect, it, vi } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import { centrePlayToggle, labelControlsWithHotkeys, type ComponentLike } from "./controls";

function component(): ComponentLike & { texts: string[]; kids: Map<string, ComponentLike> } {
  const kids = new Map<string, ComponentLike>();
  const order: ComponentLike[] = [];
  const texts: string[] = [];
  const c: ComponentLike & { texts: string[]; kids: Map<string, ComponentLike> } = {
    kids,
    texts,
    controlText: (t: string) => void texts.push(t),
    getChild: (n: string) => kids.get(n),
    children: () => order,
    removeChild: (child: ComponentLike) => void order.splice(order.indexOf(child), 1),
    addChild: (child: ComponentLike, _options: object, index: number) => void order.splice(index, 0, child),
  };
  return c;
}

describe("centrePlayToggle", () => {
  it("puts play between the skip buttons", () => {
    const bar = component();
    const play = component();
    const back = component();
    const fwd = component();
    bar.kids.set("playToggle", play).set("skipBackward", back).set("skipForward", fwd);
    bar.addChild(play, {}, 0);
    bar.addChild(back, {}, 1);
    bar.addChild(fwd, {}, 2);
    centrePlayToggle(bar);
    expect(bar.children()).toEqual([back, play, fwd]);
  });

  it("does nothing without skip buttons", () => {
    const bar = component();
    expect(() => centrePlayToggle(bar)).not.toThrow();
  });
});

describe("labelControlsWithHotkeys", () => {
  it("names the keys and follows the state", () => {
    const { player, state } = fakePlayer();
    const bar = component();
    const play = component();
    const mute = component();
    const volume = component();
    const skipBack = component();
    volume.kids.set("muteToggle", mute);
    bar.kids.set("playToggle", play).set("volumePanel", volume).set("skipBackward", skipBack);
    const stop = labelControlsWithHotkeys(player, bar, 15);
    expect(skipBack.texts).toEqual(["Seek back 15 seconds (j)"]);
    expect(play.texts).toEqual(["Play (k)"]);
    expect(mute.texts).toEqual(["Mute (m)"]);
    player.play();
    expect(play.texts.at(-1)).toBe("Pause (k)");
    state.muted = true;
    player.trigger("volumechange");
    expect(mute.texts.at(-1)).toBe("Unmute (m)");
    stop();
    player.pause();
    expect(play.texts.at(-1)).toBe("Pause (k)");
  });
});

void vi;
