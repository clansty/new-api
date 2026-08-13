const formatTimestamp = (timestamp) =>
  new Date(timestamp * 1000).toLocaleString([], {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });

export function buildPerformanceChartValues(series, metric, percentile) {
  return series
    .flatMap((item) =>
      item.points
        .filter((point) => point[metric]?.samples > 0)
        .map((point) => ({
          timestamp: point.timestamp,
          time: formatTimestamp(point.timestamp),
          value: point[metric][percentile],
          series: item.name,
        })),
    )
    .sort((left, right) => left.timestamp - right.timestamp);
}
