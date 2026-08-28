/**
 * Theme copied verbatim from web/tailwind.config.js so that SPA pages and the
 * remaining server-rendered pages resolve identical colours and spacing while both
 * exist. Keep the two in sync until web/tailwind.config.js is deleted.
 *
 * The `variants` block from the legacy config is intentionally not carried over: it
 * is Tailwind v2 syntax and has been a no-op since the v3 upgrade.
 */
export default {
  content: ["./index.html", "./src/**/*.{vue,ts}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        primary: "#0d1117",
        secondary: "#161b22",
        "secondary-light": "#353d47",
        "secondary-lighter": "#090c10",
        success: "#2fe395",
        info: "#2f56e3",
        danger: "#e3342f",
        warn: "#e3bc2f",
        wait: "#fb923c",
      },
      container: {
        center: true,
        padding: {
          DEFAULT: "1rem",
          sm: "2rem",
          lg: "6rem",
          xl: "8rem",
          "2xl": "10rem",
        },
      },
      transitionProperty: {
        width: "width",
        height: "height",
      },
      blur: {
        xxs: "1px",
      },
    },
  },
  plugins: [],
};
