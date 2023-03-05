import { VideoJsPlayer } from "video.js";

const LANGUAGES = [
    { id: "en", label: "English" },
    { id: "de", label: "Deutsch" },
];

export async function loadAndSetTrackbars(player: VideoJsPlayer, streamID: number) {
    for (const language of LANGUAGES) {
        await fetch(`/api/stream/${streamID}/subtitles/${language.id}`).then((res) => {
            if (res.ok) {
                window.dispatchEvent(new CustomEvent("togglesearch", { detail: { streamID: streamID } }));
                player.addRemoteTextTrack(
                    {
                        src: `/api/stream/${streamID}/subtitles/${language.id}`,
                        kind: "captions",
                        label: language.label,
                    },
                    false,
                );
            }
        });
    }
}
