const path = require('path');
const CopyWebpackPlugin = require('copy-webpack-plugin');

module.exports = {
  entry: './src/module.ts',
  output: {
    filename: 'module.js',
    path: path.resolve(__dirname, 'dist'),
    libraryTarget: 'amd',
    publicPath: '',
  },
  externals: {
    '@grafana/data': '@grafana/data',
    '@grafana/ui': '@grafana/ui',
    '@grafana/runtime': '@grafana/runtime',
    'react': 'react',
    'react-dom': 'react-dom',
    'rxjs': 'rxjs',
    'rxjs/operators': 'rxjs/operators',
  },
  resolve: {
    extensions: ['.ts', '.tsx', '.js'],
  },
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: 'ts-loader',
        exclude: /node_modules/,
      },
    ],
  },
  plugins: [
    new CopyWebpackPlugin({
      patterns: [
        { from: 'src/plugin.json', to: 'plugin.json' },
        { from: 'README.md', to: 'README.md' },
      ],
    }),
  ],
};
