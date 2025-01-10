import { VideoJsPlayer } from "video.js";

const LANGUAGES = [
    { id: "en", label: "English" },
    { id: "de", label: "Deutsch" },
];

export async function loadAndSetTrackbars(player: VideoJsPlayer, streamID: number) {
    for (const language of LANGUAGES) {
        await fetch(`/api/stream/${streamID}/subtitles/${language.id}`).then((res) => {
            if (res.ok) {
                window.dispatchEvent(new CustomEvent("togglesearch", { detail: { streamID: streamID } })); // Used for enabling watch searchbar
                window.dispatchEvent(new CustomEvent("toggletranscript", { detail: { streamID: streamID } })); // Used for enabling button to show transcript-modal
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
