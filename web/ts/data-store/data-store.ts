import {VideoSectionProvider} from "./video-sections";
import {BookmarksProvider} from "./bookmarks";

export abstract class DataStore {
    static bookmarks : BookmarksProvider = new BookmarksProvider();
    static videoSections : VideoSectionProvider = new VideoSectionProvider();
}
