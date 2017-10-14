var path = require('path');
module.exports = {
    entry: './src/main.js',
    output: {
        path: __dirname,
        filename: 'bundle.js'
    },
    devtool: 'inline-source-map',
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
    }
};