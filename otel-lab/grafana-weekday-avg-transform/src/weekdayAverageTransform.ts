import {
  DataFrame,
  DataTransformerID,
  Field,
  FieldType,
  MutableDataFrame,
  TransformerRegistryItem,
  getFieldDisplayName,
  standardTransformersRegistry,
  DataTransformerInfo,
} from '@grafana/data';
import { map } from 'rxjs/operators';
import { WeekdayAverageEditor } from './WeekdayAverageEditor';


// Transformation ID
export const weekdayAverageTransformId = 'weekdayAverage' as DataTransformerID;

// Granularity options
export type Granularity = 'minute' | '5min' | '15min' | '30min' | 'hour';

// Period options (in days)
export type Period = '15d' | '30d' | '60d' | '90d' | '180d' | '365d' | 'all';

// Calculation type options
export type CalculationType = 'average' | 'record_max' | 'record_min' | 'median';

// Options interface
export interface WeekdayAverageTransformOptions {
  granularity: Granularity;
  period: Period;
  seriesName: string; // Custom name for the generated series
  keepOriginal: boolean; // Keep original data
  timeField?: string;
  valueField?: string;
  sourceSeries?: string; // Selected series to calculate average from (empty = all series)
  calculationType?: CalculationType; // Type of calculation (average, record, median)
}

// Default options
export const defaultOptions: WeekdayAverageTransformOptions = {
  granularity: 'minute',
  period: '90d',
  seriesName: 'Média',
  keepOriginal: true,
  calculationType: 'average',
};

// Period to milliseconds
const periodToMs: Record<Period, number> = {
  '15d': 15 * 24 * 60 * 60 * 1000,
  '30d': 30 * 24 * 60 * 60 * 1000,
  '60d': 60 * 24 * 60 * 60 * 1000,
  '90d': 90 * 24 * 60 * 60 * 1000,
  '180d': 180 * 24 * 60 * 60 * 1000,
  '365d': 365 * 24 * 60 * 60 * 1000,
  'all': Infinity,
};

// Interface for accumulated values per time slot
interface TimeSlotAccumulator {
  values: number[]; // All values for median and record calculations
}

/**
 * Get the time slot key based on granularity
 */
function getTimeSlotKey(date: Date, granularity: Granularity): string {
  const hours = date.getHours();
  const minutes = date.getMinutes();

  switch (granularity) {
    case 'minute':
      return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}`;
    case '5min':
      return `${hours.toString().padStart(2, '0')}:${(Math.floor(minutes / 5) * 5).toString().padStart(2, '0')}`;
    case '15min':
      return `${hours.toString().padStart(2, '0')}:${(Math.floor(minutes / 15) * 15).toString().padStart(2, '0')}`;
    case '30min':
      return `${hours.toString().padStart(2, '0')}:${(Math.floor(minutes / 30) * 30).toString().padStart(2, '0')}`;
    case 'hour':
      return `${hours.toString().padStart(2, '0')}:00`;
    default:
      return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}`;
  }
}

/**
 * Find the time field in a data frame
 */
function findTimeField(frame: DataFrame, preferredName?: string): Field | undefined {
  if (preferredName) {
    const field = frame.fields.find((f) => f.name === preferredName || getFieldDisplayName(f, frame) === preferredName);
    if (field && field.type === FieldType.time) {
      return field;
    }
  }
  return frame.fields.find((f) => f.type === FieldType.time);
}

/**
 * Find a numeric value field in a data frame
 */
function findValueField(frame: DataFrame, preferredName?: string): Field | undefined {
  if (preferredName) {
    const field = frame.fields.find((f) => f.name === preferredName || getFieldDisplayName(f, frame) === preferredName);
    if (field && field.type === FieldType.number) {
      return field;
    }
  }
  return frame.fields.find((f) => f.type === FieldType.number);
}


/**
 * Calculate median from an array of numbers
 */
function calculateMedian(values: number[]): number {
  if (values.length === 0) {
    return 0;
  }
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 0) {
    return (sorted[mid - 1] + sorted[mid]) / 2;
  }
  return sorted[mid];
}

/**
 * Build a map of historical values by day of week and time slot
 * Key format: "dayOfWeek-HH:MM" (e.g., "3-15:50" for Wednesday at 15:50)
 */
function buildHistoricalValuesMap(
  timeField: Field,
  valueField: Field,
  options: WeekdayAverageTransformOptions
): Map<string, number> {
  const now = Date.now();
  const periodMs = periodToMs[options.period];
  const cutoffDate = periodMs === Infinity ? 0 : now - periodMs;
  const calculationType = options.calculationType || 'average';

  // Accumulate values by day of week + time slot
  const accumulators: Map<string, TimeSlotAccumulator> = new Map();

  for (let i = 0; i < timeField.values.length; i++) {
    const timestamp = timeField.values[i];
    const value = valueField.values[i];

    // Skip null/undefined values
    if (timestamp == null || value == null) {
      continue;
    }

    // Skip data outside the period
    if (timestamp < cutoffDate) {
      continue;
    }

    const date = new Date(timestamp);
    const dayOfWeek = date.getDay();
    const timeSlot = getTimeSlotKey(date, options.granularity);
    const key = `${dayOfWeek}-${timeSlot}`;

    const existing = accumulators.get(key) || { values: [] };
    existing.values.push(value);
    accumulators.set(key, existing);
  }

  // Calculate result based on calculation type
  const resultMap: Map<string, number> = new Map();
  for (const [key, acc] of accumulators.entries()) {
    if (acc.values.length > 0) {
      let result: number;
      switch (calculationType) {
        case 'record_max':
          result = Math.max(...acc.values);
          break;
        case 'record_min':
          result = Math.min(...acc.values);
          break;
        case 'median':
          result = calculateMedian(acc.values);
          break;
        case 'average':
        default:
          result = acc.values.reduce((sum, v) => sum + v, 0) / acc.values.length;
          break;
      }
      resultMap.set(key, result);
    }
  }

  return resultMap;
}

/**
 * Get default series name based on calculation type
 */
function getDefaultSeriesName(calculationType: CalculationType): string {
  switch (calculationType) {
    case 'record_max':
      return 'Recorde (Máximo)';
    case 'record_min':
      return 'Recorde (Mínimo)';
    case 'median':
      return 'Mediana';
    case 'average':
    default:
      return 'Média';
  }
}

/**
 * Calculate series that follows the original data timestamps
 * Returns ONE series with the calculated value for each timestamp's day of week and time slot
 */
function calculateWeekdayTimeSeries(
  timeField: Field,
  valueField: Field,
  options: WeekdayAverageTransformOptions,
  valuesMap: Map<string, number>
): DataFrame {
  const timestamps: number[] = [];
  const calculatedValues: number[] = [];

  // Generate one value for each unique timestamp in the current view
  const seenTimestamps = new Set<number>();

  for (let i = 0; i < timeField.values.length; i++) {
    const timestamp = timeField.values[i];

    if (timestamp == null || seenTimestamps.has(timestamp)) {
      continue;
    }

    seenTimestamps.add(timestamp);

    const date = new Date(timestamp);
    const dayOfWeek = date.getDay();
    const timeSlot = getTimeSlotKey(date, options.granularity);
    const key = `${dayOfWeek}-${timeSlot}`;
    const calcValue = valuesMap.get(key);

    if (calcValue !== undefined) {
      timestamps.push(timestamp);
      calculatedValues.push(calcValue);
    }
  }

  // Use the custom series name or default based on calculation type
  const calculationType = options.calculationType || 'average';
  const seriesName = options.seriesName || getDefaultSeriesName(calculationType);

  // Create result frame
  const resultFrame = new MutableDataFrame({
    name: seriesName,
    fields: [
      {
        name: 'time',
        type: FieldType.time,
        values: timestamps,
        config: {
          displayName: 'Time',
        },
      },
      {
        name: seriesName,
        type: FieldType.number,
        values: calculatedValues,
        config: {
          displayName: seriesName,
          unit: valueField.config?.unit,
          decimals: valueField.config?.decimals ?? 2,
        },
      },
    ],
  });

  return resultFrame;
}

/**
 * Get the display name of a data frame (series name)
 */
export function getFrameDisplayName(frame: DataFrame): string {
  // Try to get name from frame
  if (frame.name) {
    return frame.name;
  }
  // Try to get from refId
  if (frame.refId) {
    return frame.refId;
  }
  // Try to get from the first value field
  const valueField = frame.fields.find((f) => f.type === FieldType.number);
  if (valueField) {
    return getFieldDisplayName(valueField, frame);
  }
  return 'Unknown';
}

/**
 * Transform data frames
 */
function transformData(data: DataFrame[], options: WeekdayAverageTransformOptions): DataFrame[] {
  if (!data || data.length === 0) {
    return data;
  }

  // Merge options with defaults
  const opts: WeekdayAverageTransformOptions = {
    ...defaultOptions,
    ...options,
  };

  const results: DataFrame[] = [];

  // Keep original data if requested
  if (opts.keepOriginal) {
    results.push(...data);
  }

  // Filter frames by selected source series (if specified)
  const framesToProcess = opts.sourceSeries
    ? data.filter((frame) => getFrameDisplayName(frame) === opts.sourceSeries)
    : data;

  for (const frame of framesToProcess) {
    const timeField = findTimeField(frame, opts.timeField);
    const valueField = findValueField(frame, opts.valueField);

    if (!timeField || !valueField) {
      continue;
    }

    // Build historical values map once per frame
    const valuesMap = buildHistoricalValuesMap(timeField, valueField, opts);

    // Calculate series (automatically matches each timestamp's day of week)
    const resultFrame = calculateWeekdayTimeSeries(
      timeField,
      valueField,
      opts,
      valuesMap
    );

    // Only add if we have data
    if (resultFrame.fields[0].values.length > 0) {
      results.push(resultFrame);
    }
  }

  return results;
}

/**
 * The transformation definition
 */
const weekdayAverageTransformer: DataTransformerInfo<WeekdayAverageTransformOptions> = {
  id: weekdayAverageTransformId,
  name: 'Cálculos por Séries Temporais',
  description: 'Calcula média, mediana ou recorde histórico para cada horário baseado no mesmo dia da semana. Compare dados atuais com tendências históricas.',
  defaultOptions,

  operator: (options: WeekdayAverageTransformOptions) => (source) =>
    source.pipe(
      map((data: DataFrame[]) => transformData(data, options))
    ),
};

/**
 * Registry item for the transformation
 */
const weekdayAverageTransformRegistryItem: TransformerRegistryItem<WeekdayAverageTransformOptions> = {
  id: weekdayAverageTransformId,
  name: weekdayAverageTransformer.name,
  description: weekdayAverageTransformer.description,
  transformation: weekdayAverageTransformer,
  editor: WeekdayAverageEditor,
};

/**
 * Register the transformation with Grafana
 */
export function registerWeekdayAverageTransform(): void {
  if (standardTransformersRegistry.getIfExists(weekdayAverageTransformId)) {
    return;
  }

  standardTransformersRegistry.register(weekdayAverageTransformRegistryItem);
}
