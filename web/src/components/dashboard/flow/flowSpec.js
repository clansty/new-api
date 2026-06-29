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

// VChart Sankey spec 构建 + datum 提取, 由上游 flow.ts 1:1 移植.

const DEFAULT_FLOW_CHART_COLOR = '#1664FF';

const FLOW_NODE_KIND_SET = new Set(['user', 'node', 'token', 'group', 'model', 'channel']);

const DEFAULT_FLOW_SANKEY_LABELS = {
  quota: 'Quota',
  tokens: 'Tokens',
  requests: 'Requests',
  share: 'Share',
};

function numberValue(value) {
  const n = Number(value);
  return Number.isFinite(n) ? n : 0;
}

function isFlowNodeKind(value) {
  return typeof value === 'string' && FLOW_NODE_KIND_SET.has(value);
}

function formatNumber(value) {
  return Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value);
}

function linkStableKey(link) {
  return `${link.source}\u0000${link.target}`;
}

function byLinkDrawPriority(a, b) {
  return (
    Number(a.dimmed) - Number(b.dimmed) ||
    Number(b.highlighted) - Number(a.highlighted) ||
    b.value - a.value ||
    linkStableKey(a).localeCompare(linkStableKey(b))
  );
}

function recordValue(value) {
  return value && typeof value === 'object' ? value : undefined;
}

function sankeyDatumSource(datum) {
  const nested = datum.datum;
  if (Array.isArray(nested)) {
    const depth = numberValue(datum.depth);
    return recordValue(nested[depth]) || recordValue(nested[0]) || datum;
  }
  return recordValue(nested) || datum;
}

function sankeyDatumValue(datum, key) {
  if (datum[key] !== undefined) return datum[key];
  return sankeyDatumSource(datum)[key];
}

function sankeyDatumFlag(datum, key) {
  return sankeyDatumValue(datum, key) === true;
}

export function flowSankeyDatumValue(datum, key) {
  const record = recordValue(datum);
  return record ? sankeyDatumValue(record, key) : undefined;
}

function isSankeyLinkDatum(datum) {
  return (
    sankeyDatumValue(datum, 'source') !== undefined &&
    sankeyDatumValue(datum, 'target') !== undefined
  );
}

export function flowNodeFilterFromSankeyDatum(datum) {
  const record = recordValue(datum);
  if (!record || isSankeyLinkDatum(record)) return undefined;
  const id = flowSankeyDatumValue(record, 'key');
  const kind = flowSankeyDatumValue(record, 'kind');
  if ((typeof id === 'string' || typeof id === 'number') && isFlowNodeKind(kind)) {
    return { kind, id: String(id) };
  }
  return undefined;
}

function tooltipMetricLines(valueFormatter, labels) {
  const metricValue = (datum, key) => numberValue(sankeyDatumValue(datum, key));
  const formattedNumber = (datum, key) => formatNumber(metricValue(datum, key));
  const hasMetric = (datum, key) => metricValue(datum, key) > 0;
  return [
    { key: labels.quota, value: (datum) => valueFormatter(metricValue(datum, 'quota')) },
    { key: labels.tokens, value: (datum) => formattedNumber(datum, 'tokens') },
    { key: labels.requests, value: (datum) => formattedNumber(datum, 'requests') },
    {
      key: labels.share,
      value: (datum) => `${(metricValue(datum, 'share') * 100).toFixed(1)}%`,
      visible: (datum) => hasMetric(datum, 'share'),
    },
  ];
}

export function buildFlowSankeySpec(flow, title, valueFormatter = formatNumber, labels = DEFAULT_FLOW_SANKEY_LABELS) {
  return {
    type: 'sankey',
    data: [
      {
        id: 'flow',
        values: [
          {
            nodes: flow.nodes.map((node) => ({
              key: node.id,
              name: node.label,
              rawLabel: node.label,
              kind: node.kind,
              value: node.value,
              requests: node.requests,
              quota: node.quota,
              tokens: node.tokens,
              color: node.color,
              colorKey: node.colorKey,
              highlighted: node.highlighted,
              dimmed: node.dimmed,
            })),
            links: flow.links
              .filter((link) => link.value > 0)
              .sort(byLinkDrawPriority)
              .map((link, index) => {
                let zIndex = 100000 + index;
                if (link.highlighted) {
                  zIndex = 1000000 + index;
                } else if (link.dimmed) {
                  zIndex = index;
                }
                return {
                  source: link.source,
                  target: link.target,
                  linkKey: linkStableKey(link),
                  sourceLabel: link.sourceLabel,
                  targetLabel: link.targetLabel,
                  value: link.value,
                  requests: link.requests,
                  quota: link.quota,
                  tokens: link.tokens,
                  color: link.color,
                  linkColor: link.linkColor,
                  linkAlpha: link.linkAlpha,
                  hoverColor: link.hoverColor,
                  colorKey: link.colorKey,
                  share: link.share,
                  highlighted: link.highlighted,
                  dimmed: link.dimmed,
                  zIndex,
                };
              }),
          },
        ],
      },
    ],
    categoryField: 'name',
    sourceField: 'source',
    targetField: 'target',
    valueField: 'value',
    nodeKey: 'key',
    direction: 'horizontal',
    nodeAlign: 'justify',
    crossNodeAlign: 'middle',
    linkSortBy: (a, b) =>
      numberValue(b.value) - numberValue(a.value) ||
      `${a.source || ''}\u0000${a.target || ''}`.localeCompare(`${b.source || ''}\u0000${b.target || ''}`) ||
      numberValue(a.index) - numberValue(b.index),
    nodeGap: 14,
    nodeWidth: 16,
    minLinkHeight: 2,
    minNodeHeight: 8,
    title: { visible: false, text: title },
    legends: { visible: false },
    label: {
      visible: true,
      position: 'outside',
      limit: 220,
      interactive: false,
      style: { fill: '#475569', fontSize: 11, fontWeight: 600 },
    },
    node: {
      interactive: true,
      style: {
        fill: (datum) => String(sankeyDatumValue(datum, 'color') || DEFAULT_FLOW_CHART_COLOR),
        fillOpacity: (datum) => {
          if (sankeyDatumFlag(datum, 'dimmed')) return 0.18;
          if (sankeyDatumFlag(datum, 'highlighted')) return 1;
          return 0.92;
        },
        stroke: (datum) =>
          sankeyDatumFlag(datum, 'highlighted')
            ? 'rgba(15, 23, 42, 0.74)'
            : 'rgba(148, 163, 184, 0.45)',
        lineWidth: (datum) => (sankeyDatumFlag(datum, 'highlighted') ? 1.5 : 1),
        cursor: 'pointer',
        pickMode: 'accurate',
      },
      state: {
        hover: { fillOpacity: 1, stroke: 'rgba(15, 23, 42, 0.68)', lineWidth: 1.5 },
        selected: { fillOpacity: 1, stroke: 'rgba(15, 23, 42, 0.68)', lineWidth: 1.5 },
        blur: { fillOpacity: 0.22 },
      },
    },
    link: {
      interactive: true,
      style: {
        fill: (datum) =>
          String(sankeyDatumValue(datum, 'linkColor') || sankeyDatumValue(datum, 'color') || DEFAULT_FLOW_CHART_COLOR),
        fillOpacity: (datum) => {
          if (sankeyDatumFlag(datum, 'dimmed')) return 0.08;
          if (sankeyDatumFlag(datum, 'highlighted')) return 0.86;
          return numberValue(sankeyDatumValue(datum, 'linkAlpha')) || 1;
        },
        cursor: 'pointer',
        pickMode: 'accurate',
        boundsMode: 'accurate',
        zIndex: (datum) => {
          const zIndex = sankeyDatumValue(datum, 'zIndex');
          if (zIndex !== undefined) return numberValue(zIndex);
          return 1000000000 - numberValue(sankeyDatumValue(datum, 'value'));
        },
      },
      state: {
        hover: {
          fill: (datum) =>
            String(sankeyDatumValue(datum, 'hoverColor') || sankeyDatumValue(datum, 'color') || DEFAULT_FLOW_CHART_COLOR),
          fillOpacity: 0.9,
        },
        selected: {
          fill: (datum) =>
            String(sankeyDatumValue(datum, 'hoverColor') || sankeyDatumValue(datum, 'color') || DEFAULT_FLOW_CHART_COLOR),
          fillOpacity: 0.9,
        },
        blur: { fillOpacity: 0.22 },
      },
    },
    // 高亮完全由我们自己的 highlighted/dimmed 数据标记驱动(见上方 fillOpacity);
    // 关闭 VChart 内置的点击强调, 它的 Sankey related 处理在点击时会崩溃.
    emphasis: { enable: false },
    tooltip: {
      trigger: 'hover',
      activeType: 'mark',
      dimension: { visible: false },
      group: { visible: false },
      mark: {
        checkOverlap: true,
        positionMode: 'pointer',
        visible: (datum) =>
          isSankeyLinkDatum(datum) || sankeyDatumValue(datum, 'key') !== undefined,
        title: {
          value: (datum) => {
            const source = sankeyDatumValue(datum, 'source');
            const target = sankeyDatumValue(datum, 'target');
            if (source && target) {
              const sourceLabel = sankeyDatumValue(datum, 'sourceLabel');
              const targetLabel = sankeyDatumValue(datum, 'targetLabel');
              return `${sourceLabel || source} -> ${targetLabel || target}`;
            }
            return `${sankeyDatumValue(datum, 'name') || sankeyDatumValue(datum, 'rawLabel') || ''}`;
          },
        },
        content: tooltipMetricLines(valueFormatter, labels),
      },
    },
    background: { fill: 'transparent' },
    animation: false,
  };
}
