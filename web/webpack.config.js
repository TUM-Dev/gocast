const path = require('path');

module.exports = {
    entry: './ts/index.ts',
    module: {
        rules: [
            {
                test: /\.tsx?$/,
                use: 'ts-loader',
                exclude: /node_modules/,
            },
            {
                test: require.resolve('jquery'),
                loader: 'expose-loader',
                options:  {
                    exposes: ["$", "jQuery"]
                }
            },
            {
                test: require.resolve('moment'),
                loader: 'expose-loader',
                options:  {
                    exposes: "moment"
                }
            },
            {
                test: require.resolve('fullcalendar'),
                use: [
                    {
                        loader: 'script-loader',
                        options: 'fullcalendar/dist/fullcalendar.js'
                    }
                ]
            },
            {
                test: require.resolve('fullcalendar-scheduler'),
                use: [
                    {
                        loader: 'script-loader',
                        options: 'fullcalendar/dist/fullcalendar-scheduler.js'
                    }
                ]
            },
        ],
    },
    resolve: {
        extensions: ['.tsx', '.ts', '.js'],
    },
    output: {
        filename: 'bundle.js',
        path: path.resolve(__dirname, './assets/ts-dist'),
        library: {
            name: 'UI',
            type: 'umd',
        },
    },
    optimization: {
        minimize: false
    },

};

