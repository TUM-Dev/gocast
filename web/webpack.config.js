const path = require("path");
const MiniCssExtractPlugin = require("mini-css-extract-plugin");
const BundleAnalyzerPlugin = require("webpack-bundle-analyzer").BundleAnalyzerPlugin;

module.exports = {
    mode: "development",
    watch: true,
    entry: {
        index: "./ts/index.ts",
        video: "./ts/video.ts",
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
        library: {
            name: "UI",
            type: "umd",
        },
    },
    optimization: {
        minimize: false,
        usedExports: true,
    },
    plugins: [
        // For fullcalendar
        new MiniCssExtractPlugin({
            filename: "main.css",
        }),
        // new BundleAnalyzerPlugin(),
    ],
};
