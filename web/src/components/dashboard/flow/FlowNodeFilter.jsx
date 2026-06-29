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

import React, { useMemo } from 'react';
import { Select, Tag } from '@douyinfe/semi-ui';

const { OptGroup, Option } = Select;

// 节点筛选: 按 stage 分组的多选下拉(令牌/分组/模型/渠道), 选中项以 tag 形式展示并可移除.
const FlowNodeFilter = ({
  stages,
  stageLabels,
  metricLabel,
  formatMetricValue,
  options,
  selectedNodes,
  onChange,
  t,
}) => {
  const optionByValue = useMemo(() => {
    const map = new Map();
    for (const option of options) {
      map.set(option.value, option);
    }
    return map;
  }, [options]);

  const optionsByStage = useMemo(
    () =>
      stages
        .map((stage) => ({
          stage,
          options: options.filter((option) => option.kind === stage),
        }))
        .filter((group) => group.options.length > 0),
    [options, stages],
  );

  const selectedValues = useMemo(
    () => selectedNodes.map((node) => node.id),
    [selectedNodes],
  );

  const handleChange = (values) => {
    const next = (values || [])
      .map((id) => {
        const option = optionByValue.get(id);
        if (option) return { kind: option.kind, id: option.value };
        const existing = selectedNodes.find((node) => node.id === id);
        return existing || null;
      })
      .filter(Boolean);
    onChange(next);
  };

  const renderSelectedItem = (optionNode, { onClose }) => {
    const option = optionByValue.get(optionNode.value);
    const stageLabel = option ? stageLabels[option.kind] : '';
    const label = option ? option.label : optionNode.label || optionNode.value;
    const content = stageLabel ? `${stageLabel}: ${label}` : label;
    return {
      isRenderInTag: true,
      content: (
        <Tag
          closable
          onClose={onClose}
          size='small'
          color='white'
          style={{ maxWidth: 180 }}
        >
          <span className='truncate'>{content}</span>
        </Tag>
      ),
    };
  };

  return (
    <Select
      multiple
      filter
      value={selectedValues}
      onChange={handleChange}
      placeholder={t('全部节点')}
      maxTagCount={2}
      showClear
      ellipsisTrigger
      renderSelectedItem={renderSelectedItem}
      style={{ minWidth: 200 }}
      dropdownClassName='flow-node-filter-dropdown'
      outerTopSlot={
        <div className='px-3 py-2 text-xs text-gray-500'>
          {t('取值')}: {metricLabel}
        </div>
      }
      emptyContent={t('暂无节点')}
    >
      {optionsByStage.map((group) => (
        <OptGroup label={stageLabels[group.stage]} key={group.stage}>
          {group.options.map((option) => (
            <Option
              value={option.value}
              key={option.value}
              label={option.label}
              showTick={false}
            >
              <div className='flex items-center gap-2 w-full'>
                <span
                  className='inline-block shrink-0 rounded-full'
                  style={{
                    width: 10,
                    height: 10,
                    backgroundColor: option.color,
                  }}
                />
                <span className='flex-1 truncate'>{option.label}</span>
                <span className='shrink-0 text-xs text-gray-400 font-mono'>
                  {formatMetricValue(option.valueRaw)}
                </span>
              </div>
            </Option>
          ))}
        </OptGroup>
      ))}
    </Select>
  );
};

export default FlowNodeFilter;
