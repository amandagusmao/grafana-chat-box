const ForkTsCheckerWebpackPlugin = require('fork-ts-checker-webpack-plugin');
const { resolve } = require('path');
const CopyWebpackPlugin = require('copy-webpack-plugin');

module.exports = {
  context: resolve(__dirname, 'src'),
  entry: resolve(__dirname, 'src', 'module.ts'),
  output: {
    filename: 'module.js',
    path: resolve(__dirname, 'dist'),
    libraryTarget: 'amd',
  },
  resolve: {
    extensions: ['.ts', '.tsx', '.js', '.jsx'],
  },
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: 'ts-loader',
        exclude: /node_modules/,
      },
      {
        test: /\.css$/,
        use: ['style-loader', 'css-loader'],
      },
      {
        test: /\.scss$/,
        use: ['style-loader', 'css-loader', 'sass-loader'],
      },
    ],
  },
  plugins: [
    new ForkTsCheckerWebpackPlugin({
      typescript: {
        configFile: resolve(__dirname, 'tsconfig.json'),
      },
    }),
    new CopyWebpackPlugin({
      patterns: [
        { from: resolve(__dirname, 'src', 'plugin.json'), to: resolve(__dirname, 'dist', 'plugin.json') },
        { from: resolve(__dirname, 'src', 'img'), to: resolve(__dirname, 'dist', 'img') },
      ],
    }),
  ],
  externals: [
    'lodash',
    'moment',
    'react',
    'react-dom',
    '@grafana/ui',
    '@grafana/runtime',
    '@grafana/data',
  ],
};
