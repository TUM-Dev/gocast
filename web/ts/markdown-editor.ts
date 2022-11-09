export interface markdownEditorOptions {
    images: boolean;
    headings: boolean;
}

export interface MarkdownEditor {
    text: string;
    html: string;
    tab: string;

    update(target: HTMLTextAreaElement, pre: string, post: string);

    action(target: HTMLTextAreaElement, pre: string, post: string);

    enterHook(event: KeyboardEvent);

    isEmpty(): boolean;
}

export function markdownEditor(options: markdownEditorOptions): MarkdownEditor {
    // reList matches lines that are valid Markdown list items
    const reList = / *-.*/;
    return {
        text: "",
        html: "",
        tab: "edit",
        async update() {
            fetch("/api/markdown/preview", {
                method: "POST",
                body: JSON.stringify({markdown: this.text, options: options}),
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
        enterHook(event: KeyboardEvent) {
            // enter hook inserts a list item (- symbol) if the previous line was a list item.
            const t = event.target as HTMLTextAreaElement;
            const linesUntilEnter = t.value.substring(0, t.selectionEnd).split("\n");
            if (linesUntilEnter.length < 2) {
                return;
            }
            const lastLine = linesUntilEnter[linesUntilEnter.length - 2];
            if (reList.test(lastLine)) {
                const numIndent = lastLine.length - lastLine.trimStart().length;
                this.action(t, " ".repeat(numIndent) + "- ", "");
            }
        },
        isEmpty() {
            return this.text === "";
        }
    };
}
