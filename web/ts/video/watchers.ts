import { VideoJsPlayer } from "video.js";

/**
 * Registers a time watcher that observes the time of the current player
 * @param player: The player to register the watcher.
 * @param callback call back function responsible for handling player time updates
 * @return callBack function that got registered for listening to player time updates (used to deregister)
 */
export const registerTimeWatcher = function (
    player: VideoJsPlayer,
    callback: (currentPlayerTime: number) => void,
): () => void {
    const timeWatcherCallBack: () => void = () => callback(player.currentTime());
    player.on("timeupdate", timeWatcherCallBack);
    return timeWatcherCallBack;
};

/**
 * Deregisters a time watching observer from the current player
 * @param player The player to deregister the watcher.
 * @param callback regestered callBack function
 */
export const deregisterTimeWatcher = function (player: VideoJsPlayer, callback: () => void) {
    player.off("timeupdate", callback);
};
