import { Time } from "../global";
import { StreamableMapProvider } from "./provider";
import {Section, VideoSectionAPI} from "../api/video-sections";

export class VideoSectionProvider extends StreamableMapProvider<number, Section[]> {
    protected async fetcher(streamId: number): Promise<Section[]> {
        const result = await VideoSectionAPI.get(streamId);
        return result.map((s) => {
            s.friendlyTimestamp = new Time(s.startHours, s.startMinutes, s.startSeconds).toString();
            return s;
        });
    }
}
