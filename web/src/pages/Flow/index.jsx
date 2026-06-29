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

import React, { useCallback, useState } from 'react';
import { DatePicker, Button } from '@douyinfe/semi-ui';
import { RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { timestamp2string } from '../../helpers';
import { getRollingDashboardTimeRange } from '../../helpers/dashboard';
import { CARD_PROPS, CHART_CONFIG } from '../../constants/dashboard.constants';
import FlowPanel from '../../components/dashboard/FlowPanel';

// 分流默认查看最近 7 天.
const getDefaultFlowRange = () => {
  const now = Date.now() / 1000;
  return {
    start_timestamp: timestamp2string(now - 7 * 86400),
    end_timestamp: timestamp2string(now),
  };
};

const Flow = () => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState(() => ({
    username: '',
    ...getDefaultFlowRange(),
  }));
  const [revision, setRevision] = useState(0);

  const handleRangeChange = useCallback((dateArray) => {
    if (!Array.isArray(dateArray) || dateArray.length < 2) return;
    const [start, end] = dateArray;
    if (!start || !end) return;
    setInputs((prev) => ({
      ...prev,
      start_timestamp: timestamp2string(new Date(start).getTime() / 1000),
      end_timestamp: timestamp2string(new Date(end).getTime() / 1000),
    }));
    setRevision((r) => r + 1);
  }, []);

  const handleRefresh = useCallback(() => {
    setRevision((r) => r + 1);
  }, []);

  return (
    <div className='mt-[60px] px-2'>
      <div className='flex flex-wrap items-center justify-end gap-2 mb-3 px-1'>
        <DatePicker
          type='dateTimeRange'
          density='compact'
          value={[inputs.start_timestamp, inputs.end_timestamp]}
          onChange={handleRangeChange}
          style={{ width: 360 }}
        />
        <Button icon={<RefreshCw size={16} />} onClick={handleRefresh}>
          {t('刷新')}
        </Button>
      </div>

      <FlowPanel
        inputs={inputs}
        timeRangeRevision={revision}
        CARD_PROPS={CARD_PROPS}
        CHART_CONFIG={CHART_CONFIG}
        t={t}
      />
    </div>
  );
};

export default Flow;
