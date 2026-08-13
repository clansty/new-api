import { describe, expect, it } from 'bun:test';
import { buildPerformanceChartValues } from './chartData';

describe('buildPerformanceChartValues', () => {
  it('按时间戳全局排序跨系列数据', () => {
    // Given
    const series = [
      {
        name: 'model-a',
        points: [
          { timestamp: 3, ttft: { samples: 1, p95: 30 } },
          { timestamp: 5, ttft: { samples: 1, p95: 50 } },
        ],
      },
      {
        name: 'model-b',
        points: [
          { timestamp: 1, ttft: { samples: 1, p95: 10 } },
          { timestamp: 4, ttft: { samples: 1, p95: 40 } },
        ],
      },
    ];

    // When
    const values = buildPerformanceChartValues(series, 'ttft', 'p95');

    // Then
    expect(values.map((point) => point.timestamp)).toEqual([1, 3, 4, 5]);
    expect(values.map((point) => point.value)).toEqual([10, 30, 40, 50]);
  });
});
