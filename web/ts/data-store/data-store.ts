import { VideoSectionProvider } from "./video-sections";
import { BookmarksProvider } from "./bookmarks";
import {StreamPlaylistProvider} from "./stream-playlist";

export abstract class DataStore {
    static bookmarks: BookmarksProvider = new BookmarksProvider();
    static videoSections: VideoSectionProvider = new VideoSectionProvider();
    static streamPlaylist: StreamPlaylistProvider = new StreamPlaylistProvider();
}
