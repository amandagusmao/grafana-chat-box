const path = require('path');
const webpack = require('webpack');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const ForkTsCheckerWebpackPlugin = require('fork-ts-checker-webpack-plugin');

module.exports = (env, options) => {
  const mode = options.mode || 'development';
  const isDevelopment = mode === 'development';

  return {
    mode,
    target: 'web',
    context: path.resolve(__dirname, 'src'),
    entry: './module.tsx',
    devtool: isDevelopment ? 'eval-source-map' : 'source-map',

    output: {
      path: path.resolve(__dirname, 'dist'),
      filename: 'module.js',
      library: {
        type: 'amd'
      },
      clean: true
    },

    externals: [
      'lodash',
      'jquery',
      'moment',
      'slate',
      'prismjs',
      'slate-plain-serializer',
      'slate-react',
      'react',
      'react-dom',
      function ({ context, request }, callback) {
        const prefix = '@grafana/';
        if (request && request.indexOf(prefix) === 0) {
          return callback(null, `amd ${request}`);
        }
        callback();
      },
    ],

    plugins: [
      new ForkTsCheckerWebpackPlugin({
        typescript: {
          configFile: path.resolve(__dirname, 'tsconfig.json'),
        },
      }),
      new CopyWebpackPlugin({
        patterns: [
          { from: 'plugin.json', to: '.' },
          { from: '../README.md', to: '.' },
          { from: 'img/**/*', to: '.' },
        ],
      }),
    ],

    resolve: {
      extensions: ['.js', '.jsx', '.ts', '.tsx'],
      alias: {
        '@': path.resolve(__dirname, 'src'),
      },
    },

    module: {
      rules: [
        {
          test: /\.tsx?$/,
          use: [
            {
              loader: 'ts-loader',
              options: {
                transpileOnly: true,
              },
            },
          ],
          exclude: /node_modules/,
        },
        {
          test: /\.css$/,
          use: ['style-loader', 'css-loader'],
        },
        {
          test: /\.s[ac]ss$/,
          use: ['style-loader', 'css-loader', 'sass-loader'],
        },
        {
          test: /\.(png|jpe?g|gif|svg)$/,
          type: 'asset/resource',
        },
      ],
    },

    optimization: {
      splitChunks: false,
    },
  };
};