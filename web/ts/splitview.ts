import { getPlayers } from "./TUMLiveVjs";

export class SplitView {
    private camPercentage: number;
    private players: any[];

    showSplitMenu: boolean;

    widthPres: number;
    widthCam: number;

    static Options = {
        FullPresentation: 0,
        FocusPresentation: 25,
        SplitEvenly: 50,
        FocusCamera: 75,
        FullCamera: 100,
    };

    constructor() {
        this.camPercentage = SplitView.Options.SplitEvenly;
        this.showSplitMenu = false;
        this.players = getPlayers();
        this.updateState();
    }

    updateState() {
        this.widthCam = this.camPercentage;
        this.widthPres = 100 - this.widthCam;

        let i = 0,
            j = 1;
        if (
            this.camPercentage === SplitView.Options.FullCamera ||
            this.camPercentage === SplitView.Options.FocusCamera
        ) {
            (i = 1), (j = 0);
        }
        this.players[j].controlBar.hide();
        this.players[i].controlBar.show();
    }

    update(percentage: number) {
        this.camPercentage = percentage;
        this.updateState();
    }

    hideMenu() {
        this.showSplitMenu = false;
    }

    toggleMenu() {
        this.showSplitMenu = !this.showSplitMenu;
    }
}
