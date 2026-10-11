const path = require("path");
const MiniCssExtractPlugin = require("mini-css-extract-plugin");
const CopyWebpackPlugin = require("copy-webpack-plugin");

// Templates under web/template (the error page, the integration approval page) load a
// few packages directly as static files (Alpine, Font Awesome) instead of through a JS
// entrypoint above. Copying
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
];

module.exports = {
    mode: "development",
    target: "web",
    entry: {
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
