// eslint-disable-next-line no-undef
module.exports = {
    content: [
        "./assets/css/*.css",
        "./template/**/*.gohtml",
        "./template/**/**/*.gohtml",
        "./template/**/*.html",
        "./ts/**/*.ts",
    ],
    darkMode: "class", // or 'media' or 'false'
    theme: {
        extend: {
            // Tailwind v3's default sans stack, pinned. v4 drops `ui-sans-serif` and
            // `system-ui` from the front, which on Linux resolves to a different face
            // and shifts every text metric on the page. Delete this to adopt v4's
            // default -- a deliberate visual change, not a side effect of the upgrade.
            fontFamily: {
                sans: [
                    "ui-sans-serif",
                    "system-ui",
                    "-apple-system",
                    "BlinkMacSystemFont",
                    '"Segoe UI"',
                    "Roboto",
                    '"Helvetica Neue"',
                    "Arial",
                    '"Noto Sans"',
                    "sans-serif",
                    '"Apple Color Emoji"',
                    '"Segoe UI Emoji"',
                    '"Segoe UI Symbol"',
                    '"Noto Color Emoji"',
                ],
            },
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
