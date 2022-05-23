import videojs from "video.js";

// VideoJS uses `import * as keycode from "keycode";`
// However, keycode uses deprecated event.keyCode https://github.com/timoxley/keycode/issues/52
// The code used below avoids the mentioned IE issue (see link) by matching old key names too.

// TODO: disable some actions for live streams (forward/backwards, seek to percentage, ...)

// helpers
const clamp = (value, min, max) => Math.min(Math.max(value, min), max);
const optional = (value) => ({
    value,
    map: (ifPresent) => optional(value === undefined ? undefined : ifPresent(value)),
});
const fa = (icon) => `fa-solid fa-${icon}`;
const vjsIcon = (icon) => `vjs-icon-${icon}`;

/**
 * Checks whether the event matches one of the given key codes.
 * @param keys An iterable of strings, each naming a key code as per {@link KeyboardEvent#key}.
 * @param event The event to match.
 */
const matchKeys = (keys: Iterable<string>, event: videojs.KeyboardEvent) =>
    Array.from(keys).includes(event.key) ? event.key : undefined;

const matches = (match, player, event) =>
    typeof match === "function" ? match(player, event) : matchKeys(match, event);

const getIcon = (icon, player, event) => (typeof icon === "function" ? icon(player, event) : icon);

const handleWithClick = (name) => (player, event) => {
    const ButtonComponent = videojs.getComponent(name);
    ButtonComponent.prototype.handleClick.call(player, event);
};

function handleMute(player, event) {
    handleWithClick("MuteToggle")(player, event);
}

const handleSeek = (forward: boolean) => (player, event) => {
    const controlBar = player.controlBar;
    (forward ? controlBar.seekForward : controlBar.seekBack).handleClick(event);
};

const handleVolume = (up: boolean, step = 0.05) =>
    function (player) {
        player.volume(clamp(player.volume() + (up ? step : -step), 0, 1));
        player.muted(false);
    };

const volumeIcon = (up: boolean) => (player) =>
    vjsIcon(`volume-${player.muted() || player.volume() == 0 ? "mute" : up ? "high" : "mid"}`);

const handlePlaybackRate = (increase: boolean) => (player) => {
    const playbackRates = player.playbackRates();
    if (playbackRates) {
        const currIndex = playbackRates.indexOf(player.playbackRate());
        const newIndex = increase ? currIndex + 1 : currIndex - 1;
        if (newIndex < 0 || newIndex >= playbackRates.length) return;
        player.playbackRate(playbackRates[newIndex]);
    }
};

const numberKeys = Array.from({ length: 10 }, (_, i) => i.toString());
function matchSeekPercentage(player, event) {
    return optional(matchKeys(numberKeys, event)).map((key) => key / 10).value;
}

const handleSeekPercentage = (player, _, percentage) => {
    // TODO: what does player.duration() return for a live stream? Presumably NaN (same as before loading)
    optional(player.duration() || undefined).map((duration) => {
        const ended = player.ended();
        player.currentTime(clamp(percentage, 0, 1) * duration);
        if (ended) player.play();
    });
};

const handleSeekTo = (percentage) => (player, event) => handleSeekPercentage(player, event, percentage);

interface Hotkeys {
    [actionName: string]: { match; handle; icon? };
}

/**
 * See {@link handleHotkeys}.
 */
// TODO: is there any value to additional media key handling (MediaPause, MediaFastForward, ...)
// see https://developer.mozilla.org/en-US/docs/Web/API/UI_Events/Keyboard_event_key_values for documentation
export const defaultOptions = {
    hotkeys: {
        fullscreen: {
            match: ["f", "F"],
            handle: handleWithClick("FullscreenToggle"),
        },
        mute: {
            match: ["m", "M", "AudioVolumeMute", "VolumeMute"],
            handle: handleMute,
            icon: volumeIcon(true),
        },
        // "Spacebar" is for IE+old Firefox
        playPause: {
            // TODO: pause/play actions could depend on whether player is playing (e.g. "MediaPause" only pauses)
            match: ["k", "K", " ", "Spacebar", "MediaPause", "Pause", "MediaPlay", "MediaPlayPause"],
            handle: handleWithClick("PlayToggle"),
            icon: (player) => vjsIcon(player.paused() ? "pause" : "play"),
        },
        // "Right" is for IE+old Firefox, same with other arrows below
        seekForward: {
            match: ["l", "L", "ArrowRight", "Right"],
            handle: handleSeek(true),
            icon: `${vjsIcon("replay")} -scale-x-100 rotate-45`,
        },
        seekBack: {
            match: ["j", "J", "ArrowLeft", "Left"],
            handle: handleSeek(false),
            icon: `${vjsIcon("replay")} -rotate-45`,
        },
        volumeUp: {
            match: ["ArrowUp", "Up", "AudioVolumeUp", "VolumeUp"],
            handle: handleVolume(true),
            icon: volumeIcon(true),
        },
        volumeDown: {
            match: ["ArrowDown", "Down", "AudioVolumeDown", "VolumeDown"],
            handle: handleVolume(false),
            icon: volumeIcon(false),
        },
        increasePlaybackRate: {
            match: [">", "MediaFastForward", "PlaySpeedUp"],
            handle: handlePlaybackRate(true),
            icon: fa("forward"),
        },
        decreasePlaybackRate: {
            match: ["<", "MediaRewind", "PlaySpeedDown"],
            handle: handlePlaybackRate(false),
            icon: fa("backward"),
        },
        seekPercentage: {
            match: matchSeekPercentage,
            handle: handleSeekPercentage,
        },
        seekStart: {
            match: ["Home"],
            handle: handleSeekTo(0),
        },
        seekEnd: {
            match: ["End"],
            handle: handleSeekTo(1),
        },
    } as Hotkeys,
};

/**
 * Factory function for hotkey handler.
 * @param extraOptions Merged with {@link defaultOptions}.
 *  Each action is an entry in the `extraOptions.hotkeys` property, and replaces the default actions if given.
 *  Each value should have three properties: `match` may be an iterable of strings, in which case the action is triggered
 *  if one of those keys is pressed (as per {@link KeyboardEvent#key}), or a function, which receives as its arguments
 *  the VideoJS player and the keyboard event and should return a value if the action should be triggered, or `undefined` otherwise.
 *  `handle` must be a function that receives the VideoJS player, the event and the return value of `match`
 *  (the last of which is the {@link KeyboardEvent#key} if `match` was an iterable of strings).
 *  `icon` is optional, and may be a class string or a function (passed the same arguments as `handle`)
 *  that returns a class string.
 *  Custom actions are also supported.
 */
export function handleHotkeys(extraOptions = {}) {
    const options = videojs.mergeOptions(defaultOptions, extraOptions) as typeof defaultOptions;

    return function (event: videojs.KeyboardEvent) {
        // 'this' is the player instance
        for (const [action, { match, handle, icon }] of Object.entries(options.hotkeys)) {
            optional(matches(match, this, event)).map((data) => {
                event.preventDefault();
                event.stopPropagation();
                handle(this, event, data);
                optional(icon).map((i) => this.getChild("OverlayIcon").showIcon(getIcon(i, this, event)));
            });
        }
    };
}
