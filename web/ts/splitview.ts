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

        this.players[1].ready(() => this.setupControlBars());
        this.cloneEvents(this.players[0].el(), this.players[1].el(), ["mousemove", "mouseenter", "mouseleave"])

        // Setup splitview
        // eslint-disable-next-line @typescript-eslint/no-this-alias
        const that = this;
        this.split = Split(["#video-pres-wrapper", "#video-cam-wrapper"], {
            minSize: [0, 0],
            sizes: this.getSizes(),
            onDrag(sizes: number[]) {
                that.updateControlBarSize(sizes);
            }
        });
    }

    update(percentage: number) {
        this.camPercentage = percentage;
        const newSizes = this.getSizes();
        this.split.setSizes(newSizes);
        this.updateControlBarSize(newSizes);
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

    private cloneEvents(srcElem: HTMLElement, destElem: HTMLElement, events: string[]) {
        for (const event of events) {
            srcElem.addEventListener(event, (e) => {
                // @ts-ignore
                const clonedEvent = new e.constructor(e.type, e);
                destElem.dispatchEvent(clonedEvent);
            });
        }
    }

    private setupControlBars() {
        this.players[0].controlBar.hide();
        this.players[0].muted(true);

        const mainControlBarElem = this.players[1].controlBar.el();
        mainControlBarElem.style.position = "absolute";
        mainControlBarElem.style.zIndex = "1";
        mainControlBarElem.style.width = "100vw";

        this.updateControlBarSize(this.getSizes());
    }

    private updateControlBarSize(sizes: number[]) {
        this.players[1].controlBar.el_.style.marginLeft = `-${sizes[0]}vw`;
    }
}
