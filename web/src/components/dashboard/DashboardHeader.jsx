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
import {
  Button,
  DatePicker,
  Switch,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  RefreshCw,
  Search,
} from 'lucide-react';
import { DASHBOARD_DATE_FORMAT } from '../../helpers/dashboard';

const DashboardHeader = ({
  getGreeting,
  greetingVisible,
  showSearchModal,
  refresh,
  loading,
  dateBrowseEnabled,
  selectedDate,
  toggleDateBrowse,
  changeBrowseDate,
  shiftBrowseDate,
  t,
}) => {
  const ICON_BUTTON_CLASS = 'text-white hover:bg-opacity-80 !rounded-full';

  return (
    <div className='flex flex-col gap-3 mb-4 lg:flex-row lg:items-center lg:justify-between'>
      <h2
        className='text-2xl font-semibold text-gray-800 transition-opacity duration-1000 ease-in-out'
        style={{ opacity: greetingVisible ? 1 : 0 }}
      >
        {getGreeting}
      </h2>
      <div className='flex flex-wrap items-center gap-3'>
        <div className='flex items-center gap-2'>
          <Switch
            checked={dateBrowseEnabled}
            onChange={toggleDateBrowse}
            size='small'
            aria-label={t('日期浏览')}
          />
          <Typography.Text size='small' type='tertiary'>
            {t('日期浏览')}
          </Typography.Text>
        </div>

        {dateBrowseEnabled && (
          <div className='flex items-center gap-1 rounded-lg border border-[var(--semi-color-border)] bg-[var(--semi-color-bg-2)] px-1 py-1'>
            <Tooltip content={t('前一天')}>
              <Button
                theme='borderless'
                icon={<ChevronLeft size={16} />}
                onClick={() => shiftBrowseDate(-1)}
                aria-label={t('前一天')}
                size='small'
              />
            </Tooltip>
            <DatePicker
              type='date'
              value={selectedDate}
              onChange={(date) => changeBrowseDate(date)}
              format={DASHBOARD_DATE_FORMAT}
              inputReadOnly
              showClear={false}
              size='small'
              prefix={<CalendarDays size={14} />}
              className='w-[150px]'
              aria-label={t('选择日期')}
            />
            <Tooltip content={t('后一天')}>
              <Button
                theme='borderless'
                icon={<ChevronRight size={16} />}
                onClick={() => shiftBrowseDate(1)}
                aria-label={t('后一天')}
                size='small'
              />
            </Tooltip>
          </div>
        )}

        <Tooltip content={t('搜索条件')}>
          <Button
            type='tertiary'
            icon={<Search size={16} />}
            onClick={showSearchModal}
            className={`bg-green-500 hover:bg-green-600 ${ICON_BUTTON_CLASS}`}
            aria-label={t('搜索条件')}
          />
        </Tooltip>
        <Tooltip content={t('刷新')}>
          <Button
            type='tertiary'
            icon={<RefreshCw size={16} />}
            onClick={refresh}
            loading={loading}
            className={`bg-blue-500 hover:bg-blue-600 ${ICON_BUTTON_CLASS}`}
            aria-label={t('刷新')}
          />
        </Tooltip>
      </div>
    </div>
  );
};

export default DashboardHeader;
