if ("serviceWorker" in navigator) {
    window.addEventListener("load", () => {
        navigator.serviceWorker
            .register("/service-worker.js", { scope: "/" })
            .catch((err) => console.error("Service Worker Failed to Register", err));
    });
}

/* put this in document head to avoid FOUC */
const mediaQueryPrefersDarkScheme = window.matchMedia("(prefers-color-scheme: dark)");
function updateTheme() {
    const shouldBeDark =
        localStorage.themeMode === "dark" ||
        (localStorage.themeMode === "system" && mediaQueryPrefersDarkScheme.matches);
    if (document.documentElement.classList.contains("dark") !== shouldBeDark) {
        document.documentElement.classList.toggle("dark");
    }
}
const setTheme = (mode) => {
    localStorage.themeMode = mode;
};
const getTheme = () => localStorage.themeMode;

localStorage.removeItem("darkTheme");
// first visit or transition
if (!("themeMode" in localStorage)) {
    setTheme("system");
}

updateTheme();
mediaQueryPrefersDarkScheme.addEventListener("change", () => updateTheme());

document.addEventListener("alpine:init", () => {
    Alpine.store("theme", {
        init() {
            this.activeTheme = getTheme();
        },
        setTheme(theme) {
            this.activeTheme = theme;
            setTheme(theme);
            updateTheme();
        },
        modes: {
            light: { name: "Light", faIconId: "sun" },
            dark: { name: "Dark", faIconId: "moon" },
            system: { name: "System", faIconId: "desktop", faSwitcherIconId: "" },
        },
        activeTheme: "",
    });
});
