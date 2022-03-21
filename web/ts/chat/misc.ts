/**
 * get currently typed word based on position in the input.
 * e.g.: "hello{cursor} world" => "hello" ([0, 4])
 */
export function getCurrentWordPositions(input: string, cursorPos: number): [number, number] {
    const cursorStart = cursorPos;
    while (cursorPos > 0 && input.charAt(cursorPos - 1) !== " ") {
        cursorPos--;
    }
    return [cursorPos, cursorStart];
}