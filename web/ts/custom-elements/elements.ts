import * as help from "./help-icon";

export function defineElements() {
    customElements.define("help-icon", help.HelpIcon);
    console.log("Defined custom elements");
}
