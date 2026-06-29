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

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Card,
  RadioGroup,
  Radio,
  Select,
  Button,
  Tooltip,
  Empty,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import {
  Activity,
  ChevronRight,
  Eye,
  EyeOff,
  GitBranch,
  Hash,
  Route,
  WalletCards,
} from 'lucide-react';
import { renderQuota } from '../../helpers';
import { isAdmin, isRoot } from '../../helpers/utils';
import {
  buildDashboardFlowData,
  getDashboardChartColors,
  getFlowStages,
} from './flow/flowData';
import {
  buildFlowSankeySpec,
  flowNodeFilterFromSankeyDatum,
  flowSankeyDatumValue,
} from './flow/flowSpec';
import { compactFlowSelectionLabel, flowDisplayState } from './flow/flowSelection';
import FlowNodeFilter from './flow/FlowNodeFilter';
import { useDashboardFlow } from '../../hooks/dashboard/useDashboardFlow';

const { Text } = Typography;

const DEFAULT_FLOW_TOP_NODE_LIMIT = 50;
const FLOW_TOP_LIMIT_OPTIONS = [10, 20, 50, 100];
const MIN_VISIBLE_STAGES = 2;

const FLOW_STAGE_LABEL_KEYS = {
  user: '用户',
  node: '节点',
  token: '令牌',
  group: '分组',
  model: '模型',
  channel: '渠道',
};

const FLOW_STAGE_DESC_KEYS = {
  user: '发起请求的用户',
  node: '处理请求的部署节点',
  token: '请求使用的 API 密钥',
  group: '请求适用的用户分组',
  model: '请求的模型',
  channel: '提供服务的上游渠道',
};

const FLOW_OTHER_NODE_LABEL_KEYS = {
  user: '其他用户',
  node: '其他节点',
  token: '其他令牌',
  group: '其他分组',
  model: '其他模型',
  channel: '其他渠道',
};

const flowChartColors = getDashboardChartColors(30);

function chartRecordValue(value) {
  return value && typeof value === 'object' ? value : undefined;
}

function looksLikeFlowDatum(value) {
  const record = chartRecordValue(value);
  if (!record) return false;
  return (
    (record.key !== undefined && record.kind !== undefined) ||
    (record.source !== undefined && record.target !== undefined)
  );
}

function chartGraphicDatum(value) {
  const record = chartRecordValue(value);
  const context = chartRecordValue(record?.context);
  const data = context?.data;
  if (Array.isArray(data)) return data[0];
  return data;
}

function flowChartEventDatum(event) {
  const record = chartRecordValue(event);
  if (!record) return undefined;
  if (record.datum !== undefined && record.datum !== null) return record.datum;
  const itemRecord = chartRecordValue(record.item);
  if (itemRecord?.datum !== undefined && itemRecord.datum !== null) {
    return itemRecord.datum;
  }
  const graphicDatum = chartGraphicDatum(record.item);
  if (graphicDatum !== undefined && graphicDatum !== null) return graphicDatum;
  const itemData = itemRecord?.data;
  if (Array.isArray(itemData)) return itemData[0];
  if (itemData !== undefined && itemData !== null) return itemData;
  return looksLikeFlowDatum(record) ? record : undefined;
}

function flowNodeFilterKey(filter) {
  return `${filter.kind}\u0000${filter.id}`;
}

function isSameFlowNodeFilter(a, b) {
  return Boolean(a && a.kind === b.kind && a.id === b.id);
}

function toggleSelectedValue(values, value) {
  return values.includes(value)
    ? values.filter((item) => item !== value)
    : [...values, value];
}

function toggleSelectedNodeFilter(filters, filter) {
  const key = flowNodeFilterKey(filter);
  const hasFilter = filters.some((item) => flowNodeFilterKey(item) === key);
  return hasFilter
    ? filters.filter((item) => flowNodeFilterKey(item) !== key)
    : [...filters, filter];
}

function formatFlowMetricNumber(value) {
  return Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value);
}

const FlowPanel = ({ inputs, timeRangeRevision, CARD_PROPS, CHART_CONFIG, t }) => {
  const isAdminUser = isAdmin();
  const isRootUser = isRoot();
  let flowRole = 'user';
  if (isRootUser) {
    flowRole = 'root';
  } else if (isAdminUser) {
    flowRole = 'admin';
  }

  const { flowRows, loading, error } = useDashboardFlow({
    inputs,
    isAdminUser,
    timeRangeRevision,
  });
  const isError = Boolean(error);

  const [metric, setMetric] = useState('quota');
  const [topNodeLimit, setTopNodeLimit] = useState(DEFAULT_FLOW_TOP_NODE_LIMIT);
  const [overflowMode, setOverflowMode] = useState('aggregate');
  const [selectedUsers, setSelectedUsers] = useState([]);
  const [selectedNodes, setSelectedNodes] = useState([]);
  const [activeFlowNode, setActiveFlowNode] = useState(undefined);
  const [activeFlowLink, setActiveFlowLink] = useState(undefined);
  const [hiddenStages, setHiddenStages] = useState([]);
  const [sensitiveVisible, setSensitiveVisible] = useState(true);

  const stages = useMemo(() => getFlowStages(flowRole), [flowRole]);
  const visibleStages = useMemo(
    () => stages.filter((stage) => !hiddenStages.includes(stage)),
    [stages, hiddenStages],
  );

  useEffect(() => {
    const visible = new Set(visibleStages);
    setSelectedNodes((prev) => {
      const next = prev.filter((filter) => visible.has(filter.kind));
      return next.length === prev.length ? prev : next;
    });
    setActiveFlowNode((prev) => (prev && visible.has(prev.kind) ? prev : undefined));
    setActiveFlowLink(undefined);
  }, [visibleStages]);

  const toggleStage = useCallback(
    (stage) => {
      setHiddenStages((prev) => {
        const hidden = new Set(prev);
        if (hidden.has(stage)) {
          hidden.delete(stage);
        } else {
          const remaining = stages.filter((item) => !hidden.has(item)).length;
          if (remaining <= MIN_VISIBLE_STAGES) return prev;
          hidden.add(stage);
        }
        return stages.filter((item) => hidden.has(item));
      });
    },
    [stages],
  );

  const maskSensitive = sensitiveVisible === false;
  const flowData = useMemo(
    () =>
      buildDashboardFlowData(loading ? [] : flowRows || [], metric, {
        role: flowRole,
        colorPalette: flowChartColors,
        selectedUsers,
        selectedNodes,
        activeNode: activeFlowNode,
        activeLink: activeFlowLink,
        visibleStages,
        topNodeLimit,
        overflowMode,
        maskSensitive,
        deletedTokenLabel: (tokenId) => t('已删除 ({{id}})').replace('{{id}}', tokenId),
        otherNodeLabel: (kind) => t(FLOW_OTHER_NODE_LABEL_KEYS[kind]),
      }),
    [
      flowRole,
      flowRows,
      loading,
      metric,
      overflowMode,
      activeFlowNode,
      activeFlowLink,
      selectedNodes,
      selectedUsers,
      topNodeLimit,
      visibleStages,
      maskSensitive,
      t,
    ],
  );

  const userFilterOptions = useMemo(
    () =>
      flowData.filterOptions.users.map((user) => ({
        label: `${user.label} · ${user.valueLabel}`,
        value: user.value,
      })),
    [flowData.filterOptions.users],
  );

  const nodeFilterStages = useMemo(
    () => visibleStages.filter((stage) => stage !== 'user'),
    [visibleStages],
  );
  const nodeFilterOptions = useMemo(
    () => flowData.filterOptions.nodes.filter((option) => option.kind !== 'user'),
    [flowData.filterOptions.nodes],
  );

  const stageLabels = useMemo(() => {
    const labels = {};
    for (const stage of stages) {
      labels[stage] = t(FLOW_STAGE_LABEL_KEYS[stage]);
    }
    return labels;
  }, [stages, t]);

  const formatNodeMetricValue = useCallback(
    (value) => (metric === 'quota' ? renderQuota(value, 4) : formatFlowMetricNumber(value)),
    [metric],
  );

  const handleChartPointerDown = useCallback((event) => {
    const datum = flowChartEventDatum(event);
    const filter = flowNodeFilterFromSankeyDatum(datum);
    if (filter) {
      setActiveFlowLink(undefined);
      setActiveFlowNode((prev) => (isSameFlowNodeFilter(prev, filter) ? undefined : filter));
      return;
    }
    const source = flowSankeyDatumValue(datum, 'source');
    const target = flowSankeyDatumValue(datum, 'target');
    if (typeof source === 'string' && typeof target === 'string') {
      setActiveFlowNode(undefined);
      setActiveFlowLink((prev) =>
        prev && prev.source === source && prev.target === target
          ? undefined
          : { source, target },
      );
      return;
    }
    setActiveFlowNode(undefined);
    setActiveFlowLink(undefined);
  }, []);

  const chartTitle = t('分流');
  const flowSpec = useMemo(
    () =>
      buildFlowSankeySpec(flowData.flow, chartTitle, (v) => renderQuota(v, 4), {
        quota: t('额度'),
        tokens: t('Tokens'),
        requests: t('请求数'),
        share: t('占比'),
      }),
    [chartTitle, flowData.flow, t],
  );

  const chartKey = [
    metric,
    topNodeLimit,
    overflowMode,
    flowRole,
    activeFlowNode ? flowNodeFilterKey(activeFlowNode) : '',
    activeFlowLink ? `${activeFlowLink.source}\u0000${activeFlowLink.target}` : '',
    selectedNodes.map(flowNodeFilterKey).join(','),
    selectedUsers.join(','),
    visibleStages.join(','),
    maskSensitive ? 'masked' : 'plain',
    (flowRows || []).length,
  ].join('-');

  const displayState = flowDisplayState({
    isLoading: loading,
    isError,
    linkCount: flowData.flow.links.length,
  });

  let chartContent = (
    <VChart
      key={`flow-${chartKey}`}
      spec={{ ...flowSpec, background: 'transparent' }}
      option={CHART_CONFIG}
      onPointerDown={handleChartPointerDown}
    />
  );
  if (displayState === 'loading') {
    chartContent = (
      <div className='flex h-full items-center justify-center'>
        <Spin size='large' />
      </div>
    );
  } else if (displayState === 'error') {
    chartContent = (
      <div className='flex h-full items-center justify-center p-4'>
        <Empty title={t('加载失败')} description={error || t('请稍后重试')} />
      </div>
    );
  } else if (displayState === 'empty') {
    chartContent = (
      <div className='flex h-full items-center justify-center p-4'>
        <Empty
          image={<Route size={48} className='text-gray-300' />}
          title={t('暂无分流数据')}
          description={t('暂无数据')}
        />
      </div>
    );
  }

  const metricOptions = [
    { value: 'quota', label: t('按额度'), icon: <WalletCards size={14} /> },
    { value: 'tokens', label: t('按 Tokens'), icon: <Hash size={14} /> },
    { value: 'requests', label: t('按请求数'), icon: <Activity size={14} /> },
  ];

  return (
    <Card {...CARD_PROPS} className='!rounded-2xl'>
      <div className='flex items-center gap-2 mb-3'>
        <GitBranch size={18} className='text-gray-500' />
        <Text strong>{t('分流')}</Text>
        {loading && <Spin size='small' />}
      </div>

      <div className='flex flex-wrap items-end gap-4 mb-3'>
        <div className='flex flex-col gap-1.5'>
          <Text type='tertiary' size='small'>
            {t('连线宽度')}
          </Text>
          <RadioGroup
            type='button'
            buttonSize='small'
            value={metric}
            onChange={(e) => setMetric(e.target.value)}
          >
            {metricOptions.map((option) => (
              <Radio key={option.value} value={option.value}>
                <span className='flex items-center gap-1'>
                  {option.icon}
                  {option.label}
                </span>
              </Radio>
            ))}
          </RadioGroup>
        </div>

        <div className='flex flex-col gap-1.5'>
          <Text type='tertiary' size='small'>
            {t('显示数量')}
          </Text>
          <RadioGroup
            type='button'
            buttonSize='small'
            value={topNodeLimit}
            onChange={(e) => setTopNodeLimit(Number(e.target.value))}
          >
            {FLOW_TOP_LIMIT_OPTIONS.map((limit) => (
              <Radio key={limit} value={limit}>
                {t('前 {{count}}').replace('{{count}}', limit)}
              </Radio>
            ))}
          </RadioGroup>
        </div>

        <div className='flex flex-col gap-1.5'>
          <Text type='tertiary' size='small'>
            {t('溢出项')}
          </Text>
          <RadioGroup
            type='button'
            buttonSize='small'
            value={overflowMode}
            onChange={(e) => setOverflowMode(e.target.value)}
          >
            <Radio value='aggregate'>{t('合并为其他')}</Radio>
            <Radio value='hide'>{t('隐藏')}</Radio>
          </RadioGroup>
        </div>

        <div className='flex flex-col gap-1.5'>
          <Text type='tertiary' size='small'>
            {t('节点筛选')}
          </Text>
          <FlowNodeFilter
            stages={nodeFilterStages}
            stageLabels={stageLabels}
            metricLabel={metricOptions.find((o) => o.value === metric)?.label}
            formatMetricValue={formatNodeMetricValue}
            options={nodeFilterOptions}
            selectedNodes={selectedNodes}
            onChange={setSelectedNodes}
            t={t}
          />
        </div>

        {isAdminUser && (
          <div className='flex flex-col gap-1.5 min-w-[220px]'>
            <Text type='tertiary' size='small'>
              {t('用户筛选')}
            </Text>
            <Select
              multiple
              filter
              showClear
              maxTagCount={2}
              value={selectedUsers}
              onChange={(values) => setSelectedUsers(values || [])}
              placeholder={t('全部用户')}
              optionList={userFilterOptions}
              emptyContent={t('暂无用户')}
              style={{ minWidth: 220 }}
            />
          </div>
        )}

        <div className='flex flex-col gap-1.5'>
          <Text type='tertiary' size='small'>
            {t('敏感信息')}
          </Text>
          <Button
            size='small'
            theme='light'
            icon={sensitiveVisible ? <Eye size={14} /> : <EyeOff size={14} />}
            onClick={() => setSensitiveVisible((prev) => !prev)}
          >
            {sensitiveVisible ? t('显示') : t('隐藏')}
          </Button>
        </div>
      </div>

      <div className='rounded-lg border border-gray-200 overflow-hidden'>
        <div className='flex flex-wrap items-center gap-1.5 border-b border-gray-100 px-4 py-2'>
          <Text type='tertiary' size='small' className='mr-1'>
            {t('点击列标签可显示或隐藏该列')}
          </Text>
          {stages.map((stage, index) => {
            const visible = !hiddenStages.includes(stage);
            return (
              <React.Fragment key={stage}>
                {index > 0 && <ChevronRight size={14} className='text-gray-300' />}
                <Tooltip content={t(FLOW_STAGE_DESC_KEYS[stage])} position='top'>
                  <Button
                    size='small'
                    theme={visible ? 'light' : 'borderless'}
                    type={visible ? 'primary' : 'tertiary'}
                    onClick={() => toggleStage(stage)}
                    icon={!visible ? <EyeOff size={12} /> : undefined}
                  >
                    {stageLabels[stage]}
                  </Button>
                </Tooltip>
              </React.Fragment>
            );
          })}
        </div>
        <div style={{ height: 640 }} className='p-2'>
          {chartContent}
        </div>
      </div>
    </Card>
  );
};

export default FlowPanel;
