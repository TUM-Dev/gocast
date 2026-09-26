const path = require("path");
const MiniCssExtractPlugin = require("mini-css-extract-plugin");
const CopyWebpackPlugin = require("copy-webpack-plugin");

// Templates under web/template load a handful of packages directly as static files
// (video.js, katex, flatpickr, ...) instead of through a JS entrypoint above. Copying
// just those files into assets/vendor lets router.go embed assets/* without also
// embedding the rest of node_modules.
const vendor = (from, to) => ({ from: path.resolve(__dirname, "node_modules", from), to: path.resolve(__dirname, "assets/vendor", to) });
const vendorPatterns = [
    vendor("@fortawesome/fontawesome-free/css/all.min.css", "fontawesome/css/all.min.css"),
    vendor("@fortawesome/fontawesome-free/webfonts", "fontawesome/webfonts"),
    vendor("alpinejs/dist/cdn.js", "alpinejs/cdn.js"),
    vendor("alpinejs/dist/cdn.min.js", "alpinejs/cdn.min.js"),
    vendor("@alpinejs/persist/dist/cdn.min.js", "alpinejs-persist/cdn.min.js"),
    vendor("@alpinejs/focus/dist/cdn.min.js", "alpinejs-focus/cdn.min.js"),
    vendor("video.js/dist/video-js.min.css", "video.js/video-js.min.css"),
    vendor("video.js/dist/video.min.js", "video.js/video.min.js"),
    vendor("@silvermine/videojs-airplay/dist/silvermine-videojs-airplay.css", "videojs-airplay/silvermine-videojs-airplay.css"),
    vendor("@silvermine/videojs-airplay/dist/images", "videojs-airplay/images"),
    vendor("@videojs/http-streaming/dist/videojs-http-streaming.min.js", "videojs-http-streaming/videojs-http-streaming.min.js"),
    vendor("videojs-contrib-quality-levels/dist/videojs-contrib-quality-levels.min.js", "videojs-contrib-quality-levels/videojs-contrib-quality-levels.min.js"),
    vendor("katex/dist/katex.min.css", "katex/katex.min.css"),
    vendor("katex/dist/katex.js", "katex/katex.js"),
    vendor("katex/dist/contrib/auto-render.min.js", "katex/contrib/auto-render.min.js"),
    vendor("katex/dist/contrib/copy-tex.min.js", "katex/contrib/copy-tex.min.js"),
    vendor("katex/dist/fonts", "katex/fonts"),
    vendor("flatpickr/dist/flatpickr.min.css", "flatpickr/flatpickr.min.css"),
    vendor("flatpickr/dist/flatpickr.min.js", "flatpickr/flatpickr.min.js"),
    vendor("chart.js/dist/chart.umd.min.js", "chart.js/chart.umd.min.js"),
    vendor("nouislider/dist/nouislider.min.css", "nouislider/nouislider.min.css"),
];

module.exports = {
    mode: "development",
    target: "web",
    entry: {
        home: "./ts/entry/home.ts",
        admin: "./ts/entry/admins.ts",
        watch: "./ts/entry/video.ts",
        interaction: "./ts/entry/interactions.ts",
        global: "./ts/entry/user.ts",
    },
    module: {
        rules: [
            {
                test: /\.tsx?$/,
                use: "ts-loader",
                exclude: /node_modules/,
            },
            {
                test: require.resolve("moment"),
                loader: "expose-loader",
                options: {
                    exposes: "moment",
                },
            },
            {
                test: /\.css$/i,
                use: [
                    "handlebars-loader", // handlebars loader expects raw resource string
                    "extract-loader",
                    "css-loader",
                ],
            },
            {
                test: /\.css$/,
                use: [{ loader: MiniCssExtractPlugin.loader }, { loader: "css-loader", options: { importLoaders: 1 } }],
            },
        ],
    },
    resolve: {
        extensions: [".tsx", ".ts", ".js"],
    },
    output: {
        filename: "[name].bundle.js",
        path: path.resolve(__dirname, "./assets/ts-dist"),
        library: ["[name]"],
        libraryTarget: "umd",
    },
    plugins: [
        // For fullcalendar
        new MiniCssExtractPlugin({
            filename: "main.css",
        }),
        new CopyWebpackPlugin({ patterns: vendorPatterns }),
    ],
};
