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

import { useCallback, useEffect, useRef, useState } from 'react';
import { API } from '../../helpers';

// 分流数据获取: 复用看板的时间区间, 管理员走 /api/data/flow, 普通用户走 /api/data/flow/self.
export const useDashboardFlow = ({
  inputs,
  isAdminUser,
  timeRangeRevision,
  enabled = true,
}) => {
  const [flowRows, setFlowRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const hasLoaded = useRef(false);

  const loadFlowData = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const { start_timestamp, end_timestamp, username } = inputs;
      const localStart = Date.parse(start_timestamp) / 1000;
      const localEnd = Date.parse(end_timestamp) / 1000;
      let url = '';
      if (isAdminUser) {
        url = `/api/data/flow?username=${encodeURIComponent(username || '')}&start_timestamp=${localStart}&end_timestamp=${localEnd}`;
      } else {
        url = `/api/data/flow/self?start_timestamp=${localStart}&end_timestamp=${localEnd}`;
      }
      const res = await API.get(url);
      const { success, message, data } = res.data;
      if (success) {
        setFlowRows(data || []);
      } else {
        setError(message || '');
        setFlowRows([]);
      }
    } catch (err) {
      setError(err?.message || '');
      setFlowRows([]);
    } finally {
      setLoading(false);
    }
  }, [inputs, isAdminUser]);

  useEffect(() => {
    if (!enabled) return;
    hasLoaded.current = true;
    loadFlowData();
  }, [enabled, timeRangeRevision, loadFlowData]);

  return { flowRows, loading, error, reload: loadFlowData };
};
