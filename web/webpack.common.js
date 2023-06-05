const path = require("path");
const MiniCssExtractPlugin = require("mini-css-extract-plugin");

module.exports = {
    mode: "development",
    target: "web",
    entry: {
        home: "./ts/entry/home.ts",
        admin: "./ts/entry/admins.ts",
        watch: "./ts/entry/video.ts",
        global: "./ts/entry/user.ts",
    },
    module: {
        rules: [
            {
                test: /\.tsx?$/,
                use: "ts-loader",
                exclude: [
                    path.resolve(__dirname, "./node_modules"),
                    path.resolve(__dirname, "./ts/cypress"),
                    path.resolve(__dirname, "./ts/cypress.config.ts"),
                ],
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
    ],
};
