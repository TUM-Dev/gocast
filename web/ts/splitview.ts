export class SplitView {
    private camPercentage: number;

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
        this.updateWidths();
    }

    updateWidths() {
        this.widthCam = this.camPercentage;
        this.widthPres = 100 - this.widthCam;
    }

    update(percentage: number) {
        this.camPercentage = percentage;
        this.updateWidths();
    }

    toggleMenu() {
        this.showSplitMenu = !this.showSplitMenu;
    }
}
