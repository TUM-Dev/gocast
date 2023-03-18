import {VideoSectionProvider} from "./video-sections";

export abstract class DataStore {
    static videoSections : VideoSectionProvider = new VideoSectionProvider();
}
