import {
    Track,
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
    private room: Room;
    private screenDisplay: HTMLElement;
    private camDisplay: HTMLElement;


    constructor(streamId: number, url: string, token: string) {
        this.streamId = streamId;
        this.url = url;
        this.token = token;
        console.log("init", streamId);
    }

    public async connect(screenDisplay: HTMLElement, camDisplay: HTMLElement) {
        this.screenDisplay = screenDisplay;
        this.camDisplay = camDisplay;
        this.room = new Room({
            adaptiveStream: true,
            dynacast: true,
            videoCaptureDefaults: {
                resolution: VideoPresets.h720.resolution,
            },
        });

        await this.room.connect(this.url, this.token);
        console.log('connected to room', this.room.name);
    }

    public async enableScreen() {
        await this.room.localParticipant.setScreenShareEnabled(true);
        this.updateLocal();
    }

    public async enableCam() {
        await this.room.localParticipant.enableCameraAndMicrophone();
        this.updateLocal();
    }

    private updateLocal() {
        const tracks = this.room.localParticipant.videoTracks;
        tracks.forEach(({track, source}) => {
            console.log(track);
            if (source == Track.Source.ScreenShare) {
                const element = track.attach();
                element.style.width = "100%";
                element.style.height = "100%";

                this.screenDisplay.innerHTML = "";
                this.screenDisplay.appendChild(element);
            } else if (source == Track.Source.Camera) {
                const element = track.attach();
                element.style.width = "100%";
                element.style.height = "100%";

                this.camDisplay.innerHTML = "";
                this.camDisplay.appendChild(element);
                console.log(this.camDisplay, element);
            }
        })
    }
}