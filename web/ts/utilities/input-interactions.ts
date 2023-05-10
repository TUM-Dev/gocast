/**
 * Copies a string to the clipboard using clipboard API.
 * @param text the string that is copied to the clipboard.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
    return navigator.clipboard.writeText(text).then(
        () => {
            return true;
        },
        () => {
            return false;
        },
    );
}
