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
    private token: string;

    constructor(streamId: number, token: string) {
        this.streamId = streamId;
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

        await room.connect('ws://localhost:7800', this.token);
        console.log('connected to room', room.name);
    }
}