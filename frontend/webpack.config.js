const webpack = require("webpack");
const copyWebpackPlugin = require('copy-webpack-plugin')

const develBuild = process.env.TTY_SHARE_ENV === 'development';

let mainConfig  = {
    entry: {
        'tty-receiver': './tty-receiver/main.js',
    },
    output: {
        path: __dirname + '/public/',
        filename: '[name].js',
    },
    module: {
        rules: [
            {
                test:/\.(js|jsx)$/,
                use: [{
                    loader: 'babel-loader',
                    options: {
                      babelrc: false,
                      presets: ['env', 'react'],
                    },
                  }],
            },
            {
                test: /\.(tsx|ts)?$/,
                use: ['awesome-typescript-loader']
            },
            {
                test: /node_modules.+xterm.+\.map$/,
                use: ['ignore-loader']
            },
            {
                test: /\.scss$/,
                use: ['style-loader', 'css-loader', 'sass-loader']
            },
            {
                test: /\.css$/,
                use: ['style-loader', 'css-loader']
            },
            {
                test: /\.woff($|\?)|\.woff2($|\?)|\.ttf($|\?)|\.eot($|\?)/,
                use: ['url-loader']
            },
            {
                test: /\.(jpe?g|png|gif|svg)$/i,
                use: ['url-loader', 'image-webpack-loader']
            },
            {
                test: /\.js\.map$/,
                use: ['source-map-loader']
            }
        ]
    },
    plugins: [
        new copyWebpackPlugin([
            'static',
            'templates',
        ], {
            debug: 'info',
        }),
    ],
};

if (develBuild) {
    mainConfig.devtool = 'inline-source-map';
} else {
    mainConfig.plugins.push(new webpack.optimize.UglifyJsPlugin( { minimize: true }));
}

module.exports = mainConfig;
