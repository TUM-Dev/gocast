export function markdownEditor() {
    return {
        text: "",
        html: "",
        tab: "edit",
        async update() {
            fetch("/api/markdown", {
                method: "POST",
                body: JSON.stringify({ markdown: this.text }),
            })
                .then((response) => response.json())
                .then((data) => (this.html = data.html));
        },
        action(target: HTMLTextAreaElement, pre: string, post: string) {
            const start = target.selectionStart;
            const end = target.selectionEnd;
            target.value =
                this.text.substring(0, start) + pre + this.text.substring(start, end) + post + this.text.substring(end);
            target.selectionStart = start + pre.length;
            target.selectionEnd = end + pre.length;
            target.focus();
            this.text = target.value;
            this.update();
        },
    };
}
