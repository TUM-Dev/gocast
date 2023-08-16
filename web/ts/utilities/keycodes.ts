export function isAlphaNumeric(keyCode: number) {
    return (keyCode >= 48 && keyCode <= 57) || (keyCode >= 65 && keyCode <= 90) || (keyCode >= 97 && keyCode <= 122);
}

export function isSpacebar(keyCode: number) {
    return keyCode == 32;
}
