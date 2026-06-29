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

// 分流 Sankey 数据转换逻辑, 由上游 web/default flow.ts 1:1 移植为 JS.

const DEFAULT_FLOW_CHART_COLOR = '#1664FF';

// VChart 默认数据色板, 节点按首列(根节点)着色, 同一根的整条路径颜色稳定.
const FLOW_PALETTE = [
  '#1664FF', '#1AC6FF', '#FF8A00', '#3CC780', '#7442D4',
  '#FFC400', '#304D77', '#B48DEB', '#009488', '#FF7DDA',
  '#245EDC', '#0EA5E9', '#D97706', '#16A34A', '#9333EA',
  '#CA8A04', '#1E3A8A', '#A855F7', '#0D9488', '#DB2777',
  '#60A5FA', '#38BDF8', '#FB923C', '#4ADE80', '#C084FC',
  '#FACC15', '#475569', '#E879F9', '#2DD4BF', '#F472B6',
];

export function getDashboardChartColors(domainLength) {
  if (domainLength <= FLOW_PALETTE.length) return FLOW_PALETTE.slice();
  const colors = [];
  for (let i = 0; i < domainLength; i++) {
    colors.push(FLOW_PALETTE[i % FLOW_PALETTE.length]);
  }
  return colors;
}

const DEFAULT_FLOW_ROLE = 'user';
const DEFAULT_FLOW_OVERFLOW_MODE = 'aggregate';

const FLOW_NODE_KINDS = ['user', 'node', 'token', 'group', 'model', 'channel'];
const FLOW_NODE_KIND_SET = new Set(FLOW_NODE_KINDS);

const OTHER_FLOW_NODE_IDS = {
  user: 'user:__other__',
  node: 'node:__other__',
  token: 'token:__other__',
  group: 'group:__other__',
  model: 'model:__other__',
  channel: 'channel:__other__',
};

const DEFAULT_OTHER_FLOW_NODE_LABELS = {
  user: 'Other users',
  node: 'Other nodes',
  token: 'Other tokens',
  group: 'Other groups',
  model: 'Other models',
  channel: 'Other channels',
};

// 这些维度的标签可能泄露身份(用户/密钥/节点/分组/渠道), 脱敏时遮蔽; 模型名是公开信息, 保持可见.
const SENSITIVE_FLOW_KINDS = new Set(['user', 'node', 'token', 'group', 'channel']);

const OTHER_FLOW_NODE_ID_SET = new Set(Object.values(OTHER_FLOW_NODE_IDS));

const EMPTY_FLOW_PATH_CONTEXT = {};

function numberValue(value) {
  const n = Number(value);
  return Number.isFinite(n) ? n : 0;
}

function isFlowNodeKind(value) {
  return typeof value === 'string' && FLOW_NODE_KIND_SET.has(value);
}

function rowMetrics(row) {
  return {
    quota: numberValue(row.quota),
    tokens: numberValue(row.token_used),
    requests: numberValue(row.count),
  };
}

function metricValue(metrics, metric) {
  if (metric === 'requests') return metrics.requests;
  if (metric === 'tokens') return metrics.tokens;
  return metrics.quota;
}

function userNode(row) {
  const userID = numberValue(row.user_id);
  return {
    id: userID > 0 ? `user:${userID}` : `user:${row.username || 'unknown'}`,
    label: row.username || (userID > 0 ? `user-${userID}` : 'Unknown User'),
    kind: 'user',
  };
}

function nodeNameNode(row) {
  const nodeName = row.node_name || 'default-node';
  return { id: `node:${nodeName}`, label: nodeName, kind: 'node' };
}

function deletedTokenLabel(tokenID, ctx) {
  if (tokenID <= 0) return 'Unknown Token';
  return (ctx.deletedTokenLabel && ctx.deletedTokenLabel(tokenID)) || `token-${tokenID}`;
}

function tokenNode(row, ctx) {
  const tokenID = numberValue(row.token_id);
  return {
    id: tokenID > 0 ? `token:${tokenID}` : `token:${row.token_name || 'unknown'}`,
    label: row.token_name || deletedTokenLabel(tokenID, ctx),
    kind: 'token',
  };
}

function groupNode(row) {
  const useGroup = row.use_group || 'unknown';
  return { id: `group:${useGroup}`, label: useGroup, kind: 'group' };
}

function modelNode(row) {
  const model = row.model_name || 'unknown';
  return { id: `model:${model}`, label: row.model_name || 'Unknown Model', kind: 'model' };
}

function channelNode(row) {
  const channelID = numberValue(row.channel_id);
  return {
    id: channelID > 0 ? `channel:${channelID}` : `channel:${row.channel_name || 'unknown'}`,
    label: row.channel_name || (channelID > 0 ? `channel-${channelID}` : 'Unknown'),
    kind: 'channel',
  };
}

const NODE_BUILDERS = {
  user: userNode,
  node: nodeNameNode,
  token: tokenNode,
  group: groupNode,
  model: modelNode,
  channel: channelNode,
};

const ROLE_FLOW_STAGES = {
  root: ['user', 'node', 'token', 'group', 'model', 'channel'],
  admin: ['user', 'group', 'model', 'channel'],
  user: ['token', 'group', 'model'],
};

// Sankey 至少需要两列才能画出连线, 隐藏列时永不低于此数.
const MIN_FLOW_STAGES = 2;

export function getFlowStages(role) {
  return ROLE_FLOW_STAGES[role] || ROLE_FLOW_STAGES.user;
}

function resolveVisibleStages(role, visibleStages) {
  const stages = getFlowStages(role);
  if (!visibleStages) return stages;
  const visible = new Set(visibleStages);
  const filtered = stages.filter((stage) => visible.has(stage));
  return filtered.length >= MIN_FLOW_STAGES ? filtered : stages;
}

function flowPathForStages(row, stages, ctx = EMPTY_FLOW_PATH_CONTEXT) {
  return stages.map((stage) => NODE_BUILDERS[stage](row, ctx));
}

function colorAt(index, palette) {
  const colors = palette && palette.length > 0 ? palette : getDashboardChartColors(index + 1);
  if (colors.length === 0) return DEFAULT_FLOW_CHART_COLOR;
  return colors[index % colors.length] || DEFAULT_FLOW_CHART_COLOR;
}

function colorPalette(colorCount, palette) {
  if (palette && palette.length > 0) return palette;
  const colors = getDashboardChartColors(colorCount);
  return colors.length > 0 ? colors : [DEFAULT_FLOW_CHART_COLOR];
}

function alphaColor(color, alpha) {
  const normalized = String(color).trim();
  const hex = normalized.startsWith('#') ? normalized.slice(1) : normalized;
  if (!/^[0-9a-f]{6}$/i.test(hex)) {
    return { color: normalized, alpha };
  }
  const value = Number.parseInt(hex, 16);
  const red = (value >> 16) & 255;
  const green = (value >> 8) & 255;
  const blue = value & 255;
  return { color: `rgba(${red}, ${green}, ${blue}, ${alpha.toFixed(2)})`, alpha: 1 };
}

function stableColorMap(keys, palette) {
  const map = new Map();
  const uniqueKeys = [...new Set(keys)];
  const colors = colorPalette(uniqueKeys.length, palette);
  uniqueKeys.forEach((key, index) => {
    map.set(key, colorAt(index, colors));
  });
  return map;
}

function filterRows(rows, options = {}) {
  const selectedUsers = new Set(options.selectedUsers || []);
  if (selectedUsers.size === 0) return rows;
  return rows.filter((row) => selectedUsers.has(userNode(row).id));
}

function nodeFilterKey(filter) {
  return `${filter.kind}\u0000${filter.id}`;
}

function normalizeSelectedNodeFilters(selectedNodes, stages) {
  const visibleKinds = new Set(stages);
  const filters = new Map();
  for (const filter of selectedNodes || []) {
    if (!visibleKinds.has(filter.kind)) continue;
    const selected = filters.get(filter.kind) || new Set();
    selected.add(filter.id);
    filters.set(filter.kind, selected);
  }
  return filters;
}

function pathMatchesNodeFilters(path, filters) {
  if (filters.size === 0) return true;
  const pathNodesByKind = new Map();
  for (const node of path) {
    const ids = pathNodesByKind.get(node.kind) || new Set();
    ids.add(node.id);
    pathNodesByKind.set(node.kind, ids);
  }
  for (const [kind, selectedIds] of filters) {
    const pathIds = pathNodesByKind.get(kind);
    if (!pathIds) return false;
    let hasSelectedNode = false;
    for (const id of selectedIds) {
      if (pathIds.has(id)) {
        hasSelectedNode = true;
        break;
      }
    }
    if (!hasSelectedNode) return false;
  }
  return true;
}

function filterRowsByNodes(rows, selectedNodes, stages, ctx) {
  const filters = normalizeSelectedNodeFilters(selectedNodes, stages);
  if (filters.size === 0) return rows;
  return rows.filter((row) =>
    pathMatchesNodeFilters(flowPathForStages(row, stages, ctx), filters),
  );
}

function selectedNodeFiltersExceptKind(selectedNodes, kind) {
  const filtered = (selectedNodes || []).filter((filter) => filter.kind !== kind);
  return filtered.length > 0 ? filtered : undefined;
}

function addNode(map, pathNode, metrics, metric, color, colorKey) {
  const previous = map.get(pathNode.id) || {
    id: pathNode.id,
    label: pathNode.label,
    kind: pathNode.kind,
    value: 0,
    requests: 0,
    quota: 0,
    tokens: 0,
    color,
    colorKey,
  };
  previous.value += metricValue(metrics, metric);
  previous.requests += metrics.requests;
  previous.quota += metrics.quota;
  previous.tokens += metrics.tokens;
  map.set(pathNode.id, previous);
}

function addLink(map, source, target, metrics, metric, color, colorKey) {
  const key = `${source.id}\u0000${target.id}`;
  const previous = map.get(key) || {
    source: source.id,
    target: target.id,
    value: 0,
    requests: 0,
    quota: 0,
    tokens: 0,
    sourceLabel: source.label,
    targetLabel: target.label,
    color,
    linkColor: color,
    linkAlpha: 1,
    hoverColor: color,
    colorKey,
    share: 0,
  };
  previous.value += metricValue(metrics, metric);
  previous.requests += metrics.requests;
  previous.quota += metrics.quota;
  previous.tokens += metrics.tokens;
  map.set(key, previous);
}

function linkStableKey(link) {
  return `${link.source}\u0000${link.target}`;
}

function assignLinkDisplayColors(links) {
  const linksBySource = new Map();
  for (const link of links) {
    const sourceLinks = linksBySource.get(link.source) || [];
    sourceLinks.push(link);
    linksBySource.set(link.source, sourceLinks);
  }
  for (const sourceLinks of linksBySource.values()) {
    const sortedLinks = [...sourceLinks].sort(
      (a, b) => b.value - a.value || linkStableKey(a).localeCompare(linkStableKey(b)),
    );
    const denominator = Math.max(sortedLinks.length - 1, 1);
    sortedLinks.forEach((link, index) => {
      const alpha =
        sortedLinks.length === 1 ? 0.34 : 0.24 + (index / denominator) * 0.2;
      const displayColor = alphaColor(link.color, alpha);
      link.linkColor = displayColor.color;
      link.linkAlpha = displayColor.alpha;
      link.hoverColor = link.color;
    });
  }
}

function byValueThenLabel(a, b) {
  return b.value - a.value || a.label.localeCompare(b.label);
}

function pathLinkKey(source, target) {
  return `${source.id}\u0000${target.id}`;
}

function buildSummary(rows) {
  return rows.reduce(
    (summary, row) => {
      const metrics = rowMetrics(row);
      summary.quota += metrics.quota;
      summary.tokens += metrics.tokens;
      summary.requests += metrics.requests;
      return summary;
    },
    { quota: 0, tokens: 0, requests: 0 },
  );
}

function normalizeTopNodeLimit(limit) {
  if (limit === undefined) return undefined;
  const parsed = Math.floor(limit);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}

function otherFlowNode(kind, labeler) {
  return {
    id: OTHER_FLOW_NODE_IDS[kind],
    label: (labeler && labeler(kind)) || DEFAULT_OTHER_FLOW_NODE_LABELS[kind],
    kind,
  };
}

function buildTopNodeSets(rows, metric, stages, limit, ctx) {
  if (!limit) return undefined;
  const totals = new Map();
  for (const stage of stages) {
    totals.set(stage, new Map());
  }
  for (const row of rows) {
    const metrics = rowMetrics(row);
    const value = metricValue(metrics, metric);
    const path = flowPathForStages(row, stages, ctx);
    for (const node of path) {
      const stageTotals = totals.get(node.kind);
      if (!stageTotals) continue;
      const current = stageTotals.get(node.id) || { node, value: 0 };
      current.value += value;
      stageTotals.set(node.id, current);
    }
  }
  const topSets = new Map();
  for (const [kind, stageTotals] of totals) {
    const topIds = [...stageTotals.values()]
      .sort(
        (a, b) =>
          b.value - a.value ||
          a.node.label.localeCompare(b.node.label) ||
          a.node.id.localeCompare(b.node.id),
      )
      .slice(0, limit)
      .map((rank) => rank.node.id);
    topSets.set(kind, new Set(topIds));
  }
  return topSets;
}

function isTopFlowNode(node, topNodeSets) {
  const topNodes = topNodeSets && topNodeSets.get(node.kind);
  return !topNodes || topNodes.has(node.id);
}

function applyTopNodeLimit(path, topNodeSets, mode, labeler) {
  if (!topNodeSets) return path;
  const containsOverflowNode = path.some((node) => !isTopFlowNode(node, topNodeSets));
  if (!containsOverflowNode) return path;
  if (mode === 'hide') return undefined;
  return path.map((node) =>
    isTopFlowNode(node, topNodeSets) ? node : otherFlowNode(node.kind, labeler),
  );
}

function pathContainsFlowNode(path, filter) {
  return path.some((node) => node.kind === filter.kind && node.id === filter.id);
}

function pathContainsFlowLink(path, link) {
  for (let i = 0; i < path.length - 1; i++) {
    if (path[i] && path[i].id === link.source && path[i + 1] && path[i + 1].id === link.target) {
      return true;
    }
  }
  return false;
}

function buildFlowHighlightSets(preparedPaths, activeNode, activeLink, stages) {
  const nodeActive = Boolean(activeNode && stages.includes(activeNode.kind));
  if (!nodeActive && !activeLink) return undefined;

  const matchesPath = (path) => {
    if (activeLink) return pathContainsFlowLink(path, activeLink);
    return activeNode ? pathContainsFlowNode(path, activeNode) : false;
  };

  const highlightedNodes = new Set();
  const highlightedLinks = new Set();
  for (const prepared of preparedPaths) {
    const path = prepared.path;
    if (!matchesPath(path)) continue;
    for (const node of path) {
      highlightedNodes.add(node.id);
    }
    for (let i = 0; i < path.length - 1; i++) {
      const source = path[i];
      const target = path[i + 1];
      if (!source || !target) continue;
      highlightedLinks.add(pathLinkKey(source, target));
    }
  }
  if (highlightedNodes.size === 0) return undefined;
  return { nodes: highlightedNodes, links: highlightedLinks };
}

// 完全遮蔽标签. 节点靠 key(节点 id) 区分而非显示文本, 相同遮蔽文本不会合并.
const FLOW_MASK_TEXT = '\u2022\u2022\u2022\u2022';

function maskFlowLabel(label) {
  if (label.length === 0) return label;
  return FLOW_MASK_TEXT;
}

function maskFlowGraphLabels(nodes, links) {
  const maskedById = new Map();
  for (const node of nodes.values()) {
    if (!SENSITIVE_FLOW_KINDS.has(node.kind)) continue;
    if (OTHER_FLOW_NODE_ID_SET.has(node.id)) continue;
    const masked = maskFlowLabel(node.label);
    node.label = masked;
    maskedById.set(node.id, masked);
  }
  if (maskedById.size === 0) return;
  for (const link of links.values()) {
    const sourceMasked = maskedById.get(link.source);
    if (sourceMasked !== undefined) link.sourceLabel = sourceMasked;
    const targetMasked = maskedById.get(link.target);
    if (targetMasked !== undefined) link.targetLabel = targetMasked;
  }
}

function applyFlowHighlights(nodes, links, highlightSets) {
  if (!highlightSets) return;
  for (const node of nodes) {
    node.highlighted = highlightSets.nodes.has(node.id);
    node.dimmed = !node.highlighted;
  }
  for (const link of links) {
    const highlighted = highlightSets.links.has(linkStableKey(link));
    link.highlighted = highlighted;
    link.dimmed = !highlighted;
  }
}

function buildFlowGraph(rows, metric, role, palette, visibleStages, ctx = EMPTY_FLOW_PATH_CONTEXT, options = {}) {
  const stages = resolveVisibleStages(role, visibleStages);
  const topNodeSets = buildTopNodeSets(
    rows,
    metric,
    stages,
    normalizeTopNodeLimit(options.topNodeLimit),
    ctx,
  );
  const overflowMode = options.overflowMode || DEFAULT_FLOW_OVERFLOW_MODE;
  const preparedPaths = [];

  for (const row of rows) {
    const path = applyTopNodeLimit(
      flowPathForStages(row, stages, ctx),
      topNodeSets,
      overflowMode,
      options.otherNodeLabel,
    );
    if (!path) continue;
    preparedPaths.push({ path, metrics: rowMetrics(row) });
  }

  const nodes = new Map();
  const links = new Map();
  const colors = stableColorMap(
    preparedPaths
      .map((prepared) => prepared.path[0] && prepared.path[0].id)
      .filter((id) => Boolean(id))
      .sort((a, b) => a.localeCompare(b)),
    palette,
  );

  for (const prepared of preparedPaths) {
    const path = prepared.path;
    const metrics = prepared.metrics;
    const root = path[0];
    if (!root) continue;
    const color = colors.get(root.id) || colorAt(0, palette);

    for (const node of path) {
      addNode(nodes, node, metrics, metric, color, root.id);
    }
    for (let i = 0; i < path.length - 1; i++) {
      const source = path[i];
      const target = path[i + 1];
      if (!source || !target) continue;
      addLink(links, source, target, metrics, metric, color, root.id);
    }
  }
  if (options.maskSensitive) {
    maskFlowGraphLabels(nodes, links);
  }
  applyFlowHighlights(
    nodes.values(),
    links.values(),
    buildFlowHighlightSets(preparedPaths, options.activeNode, options.activeLink, stages),
  );

  const flowLinks = [...links.values()].sort(
    (a, b) => a.source.localeCompare(b.source) || a.target.localeCompare(b.target),
  );
  const firstStepSources = new Set(
    preparedPaths
      .map((prepared) => prepared.path[0] && prepared.path[0].id)
      .filter((id) => Boolean(id)),
  );
  const total = flowLinks
    .filter((link) => firstStepSources.has(link.source))
    .reduce((sum, link) => sum + link.value, 0);
  for (const link of flowLinks) {
    link.share = total > 0 ? link.value / total : 0;
  }
  assignLinkDisplayColors(flowLinks);

  return {
    nodes: [...nodes.values()].sort(byValueThenLabel),
    links: flowLinks,
  };
}

function formatNumber(value) {
  return Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value);
}

function buildUserFilterOptions(rows, metric = 'quota', palette) {
  const users = new Map();
  const colors = stableColorMap(
    rows.map((row) => userNode(row).id).sort((a, b) => a.localeCompare(b)),
    palette,
  );
  for (const row of rows) {
    const user = userNode(row);
    if (!row.user_id && !row.username) continue;
    const metrics = rowMetrics(row);
    const value = metricValue(metrics, metric);
    const current = users.get(user.id) || {
      label: user.label,
      value: 0,
      color: colors.get(user.id) || colorAt(0, palette),
    };
    current.value += value;
    users.set(user.id, current);
  }
  return [...users.entries()]
    .map(([value, user]) => ({
      value,
      label: user.label,
      valueLabel: formatNumber(user.value),
      valueRaw: user.value,
      color: user.color,
    }))
    .sort((a, b) => b.valueRaw - a.valueRaw || a.label.localeCompare(b.label));
}

function buildNodeFilterOptions(rows, metric, role, visibleStages, palette, ctx, selectedNodes) {
  const stages = resolveVisibleStages(role, visibleStages);
  const stageOrder = new Map(stages.map((stage, index) => [stage, index]));
  const colorIds = new Set();
  for (const row of rows) {
    for (const node of flowPathForStages(row, stages, ctx)) {
      colorIds.add(node.id);
    }
  }
  const colors = stableColorMap([...colorIds].sort((a, b) => a.localeCompare(b)), palette);
  const options = [];

  for (const stage of stages) {
    const totals = new Map();
    const candidateRows = filterRowsByNodes(
      rows,
      selectedNodeFiltersExceptKind(selectedNodes, stage),
      stages,
      ctx,
    );
    for (const row of candidateRows) {
      const metrics = rowMetrics(row);
      const value = metricValue(metrics, metric);
      const node = NODE_BUILDERS[stage](row, ctx);
      const key = nodeFilterKey({ kind: node.kind, id: node.id });
      const current = totals.get(key) || { node, value: 0 };
      current.value += value;
      totals.set(key, current);
    }
    for (const rank of totals.values()) {
      options.push({
        kind: rank.node.kind,
        value: rank.node.id,
        label: rank.node.label,
        valueLabel: formatNumber(rank.value),
        valueRaw: rank.value,
        color: colors.get(rank.node.id) || colorAt(0, palette),
      });
    }
  }

  return options.sort(
    (a, b) =>
      (stageOrder.get(a.kind) || 0) - (stageOrder.get(b.kind) || 0) ||
      b.valueRaw - a.valueRaw ||
      a.label.localeCompare(b.label) ||
      a.value.localeCompare(b.value),
  );
}

export function buildDashboardFlowData(rows, metric = 'quota', options = {}) {
  const role = options.role || DEFAULT_FLOW_ROLE;
  const palette = options.colorPalette;
  const ctx = { deletedTokenLabel: options.deletedTokenLabel };
  const stages = resolveVisibleStages(role, options.visibleStages);
  const userFilteredRows = filterRows(rows, options);
  const filteredRows = filterRowsByNodes(userFilteredRows, options.selectedNodes, stages, ctx);

  return {
    summary: buildSummary(filteredRows),
    flow: buildFlowGraph(filteredRows, metric, role, palette, options.visibleStages, ctx, {
      topNodeLimit: options.topNodeLimit,
      overflowMode: options.overflowMode,
      otherNodeLabel: options.otherNodeLabel,
      activeNode: options.activeNode,
      activeLink: options.activeLink,
      maskSensitive: options.maskSensitive,
    }),
    filterOptions: {
      users: buildUserFilterOptions(rows, metric, palette),
      nodes: buildNodeFilterOptions(
        userFilteredRows,
        metric,
        role,
        options.visibleStages,
        palette,
        ctx,
        options.selectedNodes,
      ),
    },
  };
}

export { formatNumber };
