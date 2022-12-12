import { getPlayers } from "./TUMLiveVjs";
import Split from "split.js";

export class SplitView {
    private camPercentage: number;
    private players: any[];
    private split: Split.Instance;

    showSplitMenu: boolean;

    static Options = {
        FullPresentation: 0,
        FocusPresentation: 25,
        SplitEvenly: 50,
        FocusCamera: 75,
        FullCamera: 100,
    };

    constructor() {
        this.camPercentage = SplitView.Options.FocusPresentation;
        this.showSplitMenu = false;
        this.players = getPlayers();
        this.toggleControlBars(this.camPercentage);

        // Setup splitview
        // eslint-disable-next-line @typescript-eslint/no-this-alias
        const that = this;
        this.split = Split(["#video-pres-wrapper", "#video-cam-wrapper"], {
            minSize: [0, 0],
            sizes: this.getSizes(),
            onDragEnd: function (sizes: number[]) {
                that.toggleControlBars(sizes[1]);
            },
        });
    }

    update(percentage: number) {
        this.camPercentage = percentage;
        this.split.setSizes(this.getSizes());
        this.toggleControlBars(this.camPercentage);
    }

    hideMenu() {
        this.showSplitMenu = false;
    }

    toggleMenu() {
        this.showSplitMenu = !this.showSplitMenu;
    }

    private getSizes(): number[] {
        return [100 - this.camPercentage, this.camPercentage];
    }

    private toggleControlBars(camPercentage: number) {
        let i = 0,
            j = 1;
        if (camPercentage > 50) {
            (i = 1), (j = 0);
        }
        this.players[j].controlBar.hide();
        this.players[i].controlBar.show();
        this.players[j].muted(true);
        this.players[i].muted(false);
    }
}
