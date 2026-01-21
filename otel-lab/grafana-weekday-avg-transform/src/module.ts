import { AppPlugin } from '@grafana/data';
import { registerWeekdayAverageTransform } from './weekdayAverageTransform';

// Register the transformation when the plugin loads
registerWeekdayAverageTransform();

// Export a minimal app plugin (required for plugin to load)
export const plugin = new AppPlugin();
