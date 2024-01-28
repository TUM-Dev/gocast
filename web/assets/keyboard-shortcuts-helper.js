function openKeyboardShortcutsPopup() {
    var popup = document.getElementById("keyboardShortcutsPopup");
    popup.style.display = "flex";
    console.log("test1");
    document.addEventListener("keydown", handleKeyDown);
}

function closeKeyboardShortcutsPopup() {
    var popup = document.getElementById("keyboardShortcutsPopup");
    popup.style.display = "none";
    document.removeEventListener("keydown", handleKeyDown);
}

function handleKeyDown(event) {
    if (event.key === "Escape") {
        closeKeyboardShortcutsPopup();
    }
}

document.addEventListener("DOMContentLoaded", function () {
    // Get the overlay element
    var overlay = document.getElementById("keyboardShortcutsPopup");

    // Add click event listener to the overlay
    overlay.addEventListener("click", function (event) {
        // If the clicked element is the overlay itself, close the popup
        if (event.target === overlay) {
            closeKeyboardShortcutsPopup();
        }
    });
});