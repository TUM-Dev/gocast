import {StreamableMapProvider} from "./provider";
import {Subtitle, SubtitleAPI} from "../api/subtitles";

export class SubtitleProvider extends StreamableMapProvider<number, boolean> {
    protected async fetcher(streamId: number): Promise<boolean> {
        return await SubtitleAPI.getPresent(streamId)
    }
}
