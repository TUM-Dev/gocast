import {
    Participant,
    RemoteParticipant,
    RemoteTrack,
    RemoteTrackPublication,
    Room,
    RoomEvent,
    VideoPresets,
} from 'livekit-client';

export class Livestream {
    private streamId: number;
    private url: string;
    private token: string;

    constructor(streamId: number, url: string, token: string) {
        this.streamId = streamId;
        this.url = url;
        this.token = token;
        console.log("init", streamId);
    }

    public async connect() {
        const room = new Room({
            adaptiveStream: true,
            dynacast: true,
            videoCaptureDefaults: {
                resolution: VideoPresets.h720.resolution,
            },
        });

        await room.connect(this.url, this.token);
        console.log('connected to room', room.name);
    }
}