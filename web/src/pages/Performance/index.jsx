/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Empty,
  Radio,
  RadioGroup,
  Select,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import { Activity, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';
import { buildPerformanceChartValues } from './chartData';

const RANGE_SECONDS = { day: 86400, week: 604800, month: 2592000 };
const SMALL_SIZE = 'small';

function formatMetric(value, metric) {
  if (!Number.isFinite(value)) return '-';
  if (metric === 'tps') return `${value.toFixed(value >= 100 ? 0 : 1)} tok/s`;
  return value >= 1000
    ? `${(value / 1000).toFixed(2)}s`
    : `${value.toFixed(0)}ms`;
}

function formatChannel(name, id) {
  return `${name} (#${id})`;
}

const Performance = () => {
  const { t } = useTranslation();
  const [metric, setMetric] = useState('ttft');
  const [percentile, setPercentile] = useState('p95');
  const [groupBy, setGroupBy] = useState('model');
  const [range, setRange] = useState('day');
  const [modelName, setModelName] = useState('');
  const [channel, setChannel] = useState(0);
  const [revision, setRevision] = useState(0);
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState({
    series: [],
    models: [],
    channels: [],
    summary: { requests: 0, ttft: {}, tps: {} },
  });

  useEffect(() => {
    initVChartSemiTheme({ isWatchingThemeSwitch: true });
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    const endTimestamp = Math.floor(Date.now() / 1000);
    setLoading(true);
    API.get('/api/log/performance', {
      signal: controller.signal,
      disableDuplicate: true,
      skipErrorHandler: true,
      params: {
        start_timestamp: endTimestamp - RANGE_SECONDS[range],
        end_timestamp: endTimestamp,
        model_name: modelName,
        channel,
        group_by: groupBy,
      },
    })
      .then((response) => {
        if (response.data.success) {
          setData(response.data.data);
          return;
        }
        showError(response.data.message);
      })
      .catch((error) => {
        if (error.code !== 'ERR_CANCELED') showError(error.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [range, modelName, channel, groupBy, revision]);

  const chartValues = useMemo(
    () => buildPerformanceChartValues(data.series, metric, percentile),
    [data.series, metric, percentile],
  );

  const chartSpec = useMemo(
    () => ({
      type: 'line',
      data: [{ id: 'performance', values: chartValues }],
      xField: 'time',
      yField: 'value',
      seriesField: 'series',
      padding: { top: 24, right: 22, bottom: 12, left: 18 },
      point: { visible: true, style: { size: 5 } },
      line: { style: { lineWidth: 2 } },
      axes: [
        { orient: 'bottom', label: { autoRotate: true, autoHide: true } },
        {
          orient: 'left',
          title: { visible: true, text: metric === 'tps' ? 'tok/s' : 'ms' },
          label: {
            formatMethod: (value) => formatMetric(Number(value), metric),
          },
        },
      ],
      legends: { visible: true, orient: 'bottom', maxRow: 2 },
      tooltip: {
        dimension: {
          content: [
            {
              key: (datum) => datum.series,
              value: (datum) => formatMetric(datum.value, metric),
            },
          ],
        },
      },
    }),
    [chartValues, metric],
  );

  const summary = data.summary[metric] || {};
  const cards = [
    [t('请求数'), data.summary.requests?.toLocaleString() || '0'],
    ['p50', formatMetric(summary.p50, metric)],
    ['p95', formatMetric(summary.p95, metric)],
    ['p99', formatMetric(summary.p99, metric)],
  ];

  return (
    <div className='mt-[60px] px-2 pb-6'>
      <div className='mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
        <div className='flex items-center gap-2'>
          <Activity size={20} />
          <Typography.Title heading={4}>{t('性能分析')}</Typography.Title>
          <Typography.Text type='tertiary'>
            {t('按消费日志统计')}
          </Typography.Text>
        </div>
        <Button
          icon={<RefreshCw size={16} />}
          loading={loading}
          onClick={() => setRevision((value) => value + 1)}
        >
          {t('刷新')}
        </Button>
      </div>

      <div className='mb-3 flex flex-wrap items-end gap-3'>
        <Control label={t('指标')}>
          <RadioGroup
            type='button'
            buttonSize={SMALL_SIZE}
            value={metric}
            onChange={(event) => setMetric(event.target.value)}
          >
            <Radio value='ttft'>{t('TTFT')}</Radio>
            <Radio value='tps'>{t('TPS')}</Radio>
          </RadioGroup>
        </Control>
        <Control label={t('百分位')}>
          <RadioGroup
            type='button'
            buttonSize={SMALL_SIZE}
            value={percentile}
            onChange={(event) => setPercentile(event.target.value)}
          >
            <Radio value='p50'>{t('p50')}</Radio>
            <Radio value='p95'>{t('p95')}</Radio>
            <Radio value='p99'>{t('p99')}</Radio>
          </RadioGroup>
        </Control>
        <Control label={t('分组')}>
          <RadioGroup
            type='button'
            buttonSize={SMALL_SIZE}
            value={groupBy}
            onChange={(event) => setGroupBy(event.target.value)}
          >
            <Radio value='model'>{t('按模型')}</Radio>
            <Radio value='channel'>{t('按渠道')}</Radio>
          </RadioGroup>
        </Control>
        <Select
          size='small'
          value={modelName}
          onChange={setModelName}
          className='w-44'
          aria-label={t('模型')}
        >
          <Select.Option value=''>{t('全部模型')}</Select.Option>
          {data.models.map((model) => (
            <Select.Option key={model} value={model}>
              {model}
            </Select.Option>
          ))}
        </Select>
        <Select
          size='small'
          value={channel}
          onChange={setChannel}
          className='w-44'
          aria-label={t('渠道')}
        >
          <Select.Option value={0}>{t('全部渠道')}</Select.Option>
          {data.channels.map((item) => (
            <Select.Option key={item.id} value={item.id}>
              {formatChannel(item.name, item.id)}
            </Select.Option>
          ))}
        </Select>
        <RadioGroup
          type='button'
          buttonSize={SMALL_SIZE}
          value={range}
          onChange={(event) => setRange(event.target.value)}
        >
          <Radio value='day'>{t('最近一天')}</Radio>
          <Radio value='week'>{t('最近七天')}</Radio>
          <Radio value='month'>{t('最近三十天')}</Radio>
        </RadioGroup>
      </div>

      <Card className='!rounded-lg' bodyStyle={{ padding: 12 }}>
        <div className='h-[420px] min-h-[320px]'>
          {loading ? (
            <div className='flex h-full items-center justify-center'>
              <Spin size='large' />
            </div>
          ) : chartValues.length > 0 ? (
            <VChart spec={chartSpec} />
          ) : (
            <Empty className='pt-28' title={t('暂无性能数据')} />
          )}
        </div>
      </Card>

      <div className='mt-3 grid grid-cols-2 gap-3 lg:grid-cols-4'>
        {cards.map(([label, value]) => (
          <Card key={label} className='!rounded-lg' bodyStyle={{ padding: 16 }}>
            <Typography.Text type='tertiary'>{label}</Typography.Text>
            <div className='mt-1 font-mono text-xl font-semibold'>{value}</div>
          </Card>
        ))}
      </div>
    </div>
  );
};

const Control = ({ label, children }) => (
  <div className='flex flex-col gap-1'>
    <Typography.Text size='small' type='tertiary'>
      {label}
    </Typography.Text>
    {children}
  </div>
);

export default Performance;
