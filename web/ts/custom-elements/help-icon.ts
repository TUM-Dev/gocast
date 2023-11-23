export class HelpIcon extends HTMLElement {
    private text: string;
    constructor() {
        super();
    }

    connectedCallback() {
        this.text = this.getAttribute("text") ?? "No help available";
        this.innerHTML = `
            <style>
                .help-icon-tooltip-desktop {
                    max-width: 22rem;
                }
            </style>
            <span
                x-data="{ tooltip: false }" 
                x-on:mouseover="tooltip = true" 
                x-on:mouseleave="tooltip = false"
                class="cursor-pointer m-0 pl-1 text-sm">
              <i class="fa-solid fa-circle-info text-gray-700 dark:text-gray-400 w-fit h-fit justify-self-center"></i>
              <div x-show="tooltip"
                class="text-sm help-icon-tooltip-desktop text-white absolute primary rounded-lg p-1.5"
              >
                 ${this.text}
              </div>
            </span>
        `;
        this.className = "m-0 p-0 text-xs";
        this.style.textRendering = "";
    }
}