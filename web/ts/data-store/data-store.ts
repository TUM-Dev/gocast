import { VideoSectionProvider } from "./video-sections";
import { BookmarksProvider } from "./bookmarks";
import { StreamPlaylistProvider } from "./stream-playlist";
import {AdminLectureListProvider} from "./admin-lecture-list";

export abstract class DataStore {
    static bookmarks: BookmarksProvider = new BookmarksProvider();
    static videoSections: VideoSectionProvider = new VideoSectionProvider();
    static streamPlaylist: StreamPlaylistProvider = new StreamPlaylistProvider();

    // Admin Data-Stores
    static adminLectureList: AdminLectureListProvider = new AdminLectureListProvider();
}
