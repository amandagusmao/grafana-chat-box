import React, { ChangeEvent } from 'react';
import { TransformerUIProps, SelectableValue } from '@grafana/data';
import { InlineField, Select, InlineFieldRow, Input, InlineSwitch } from '@grafana/ui';
import { WeekdayAverageTransformOptions, Granularity, Period } from './weekdayAverageTransform';

const granularityOptions: Array<SelectableValue<Granularity>> = [
  { label: 'Every minute', value: 'minute' },
  { label: 'Every 5 minutes', value: '5min' },
  { label: 'Every 15 minutes', value: '15min' },
  { label: 'Every 30 minutes', value: '30min' },
  { label: 'Every hour', value: 'hour' },
];

const periodOptions: Array<SelectableValue<Period>> = [
  { label: 'Last 15 days', value: '15d' },
  { label: 'Last 30 days', value: '30d' },
  { label: 'Last 60 days', value: '60d' },
  { label: 'Last 90 days (3 months)', value: '90d' },
  { label: 'Last 180 days (6 months)', value: '180d' },
  { label: 'Last 365 days (1 year)', value: '365d' },
  { label: 'All available data', value: 'all' },
];

export function WeekdayAverageEditor({
  options,
  onChange,
}: TransformerUIProps<WeekdayAverageTransformOptions>) {
  const onGranularityChange = (value: SelectableValue<Granularity>) => {
    onChange({
      ...options,
      granularity: value?.value ?? 'minute',
    });
  };

  const onPeriodChange = (value: SelectableValue<Period>) => {
    onChange({
      ...options,
      period: value?.value ?? '90d',
    });
  };

  const onSeriesNameChange = (e: ChangeEvent<HTMLInputElement>) => {
    onChange({
      ...options,
      seriesName: e.target.value,
    });
  };

  const onKeepOriginalChange = (e: ChangeEvent<HTMLInputElement>) => {
    onChange({
      ...options,
      keepOriginal: e.target.checked,
    });
  };

  const currentGranValue = granularityOptions.find((o) => o.value === options.granularity) || granularityOptions[0];
  const currentPeriodValue = periodOptions.find((o) => o.value === options.period) || periodOptions[3];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
      <div style={{ display: 'flex', gap: '16px' }}>
        <InlineField label="Period" labelWidth={16}>
          <Select<Period>
            width={25}
            options={periodOptions}
            value={currentPeriodValue}
            onChange={onPeriodChange}
          />
        </InlineField>
        <InlineField label="Granularity" labelWidth={16}>
          <Select<Granularity>
            width={20}
            options={granularityOptions}
            value={currentGranValue}
            onChange={onGranularityChange}
          />
        </InlineField>
      </div>

      <div style={{ display: 'flex', gap: '16px' }}>
        <InlineField label="Series Name" labelWidth={16}>
          <Input
            width={25}
            value={options.seriesName || 'Média'}
            onChange={onSeriesNameChange}
            placeholder="Média"
          />
        </InlineField>
        <InlineField label="Keep Original" labelWidth={16}>
          <InlineSwitch
            value={options.keepOriginal !== false}
            onChange={onKeepOriginalChange}
          />
        </InlineField>
      </div>

      <div style={{ marginTop: '8px', color: '#999', fontSize: '12px' }}>
        Calcula a média histórica para cada horário baseado no mesmo dia da semana.
      </div>
    </div>
  );
}
