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

import React from 'react';
import { Card, Select, Switch, Typography } from '@douyinfe/semi-ui';
import { PieChart } from 'lucide-react';
import { VChart } from '@visactor/react-vchart';

const ChartsPanel = ({
  activeChartTab,
  setActiveChartTab,
  spec_line,
  spec_model_line,
  spec_pie,
  spec_rank_bar,
  spec_token_bar,
  spec_token_pie,
  spec_channel_bar,
  spec_channel_pie,
  spec_cost_bar,
  spec_channel_cost_bar,
  spec_user_rank,
  spec_user_trend,
  CARD_PROPS,
  CHART_CONFIG,
  FLEX_CENTER_GAP2,
  hasApiInfoPanel,
  isAdminUser,
  showAllTokens,
  onToggleAllTokens,
  t,
}) => {
  const chartOptions = [
    { value: '1', label: t('消耗分布') },
    { value: '2', label: t('调用趋势') },
    { value: '3', label: t('调用次数分布') },
    { value: '4', label: t('调用次数排行') },
    { value: '5', label: t('令牌消耗分布') },
    { value: '6', label: t('令牌消耗占比') },
    { value: '7', label: t('渠道消耗分布'), adminOnly: true },
    { value: '8', label: t('渠道消耗占比'), adminOnly: true },
    { value: '9', label: t('用户消耗排行'), adminOnly: true },
    { value: '10', label: t('用户消耗趋势'), adminOnly: true },
    { value: '11', label: t('成本分布'), adminOnly: true },
    { value: '12', label: t('渠道成本分布'), adminOnly: true },
  ].filter((option) => isAdminUser || !option.adminOnly);

  return (
    <Card
      {...CARD_PROPS}
      className={`!rounded-2xl ${hasApiInfoPanel ? 'lg:col-span-3' : ''}`}
      title={
        <div className='flex flex-col lg:flex-row lg:items-center lg:justify-between w-full gap-3'>
          <div className={FLEX_CENTER_GAP2}>
            <PieChart size={16} />
            {t('模型数据分析')}
            {isAdminUser && (
              <div className='ml-4 flex items-center gap-2'>
                <Switch
                  checked={showAllTokens}
                  onChange={onToggleAllTokens}
                  size='small'
                />
                <Typography.Text size='small' type='tertiary'>
                  {t('所有令牌')}
                </Typography.Text>
              </div>
            )}
          </div>
          <Select
            value={activeChartTab}
            onChange={setActiveChartTab}
            optionList={chartOptions}
            className='w-full lg:w-56'
            aria-label={t('模型数据分析')}
          />
        </div>
      }
      bodyStyle={{ padding: 0 }}
    >
      <div className='h-96 p-2'>
        {activeChartTab === '1' && (
          <VChart spec={spec_line} option={CHART_CONFIG} />
        )}
        {activeChartTab === '2' && (
          <VChart spec={spec_model_line} option={CHART_CONFIG} />
        )}
        {activeChartTab === '3' && (
          <VChart spec={spec_pie} option={CHART_CONFIG} />
        )}
        {activeChartTab === '4' && (
          <VChart spec={spec_rank_bar} option={CHART_CONFIG} />
        )}
        {activeChartTab === '5' && (
          <VChart spec={spec_token_bar} option={CHART_CONFIG} />
        )}
        {activeChartTab === '6' && (
          <VChart spec={spec_token_pie} option={CHART_CONFIG} />
        )}
        {activeChartTab === '7' && isAdminUser && (
          <VChart spec={spec_channel_bar} option={CHART_CONFIG} />
        )}
        {activeChartTab === '8' && isAdminUser && (
          <VChart spec={spec_channel_pie} option={CHART_CONFIG} />
        )}
        {activeChartTab === '9' && isAdminUser && (
          <VChart spec={spec_user_rank} option={CHART_CONFIG} />
        )}
        {activeChartTab === '10' && isAdminUser && (
          <VChart spec={spec_user_trend} option={CHART_CONFIG} />
        )}
        {activeChartTab === '11' && isAdminUser && (
          <VChart spec={spec_cost_bar} option={CHART_CONFIG} />
        )}
        {activeChartTab === '12' && isAdminUser && (
          <VChart spec={spec_channel_cost_bar} option={CHART_CONFIG} />
        )}
      </div>
    </Card>
  );
};

export default ChartsPanel;
