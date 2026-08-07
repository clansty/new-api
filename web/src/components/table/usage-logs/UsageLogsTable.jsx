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
import { Empty, Descriptions } from '@douyinfe/semi-ui';
import CardTable from '../../common/ui/CardTable';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { getLogsColumns } from './UsageLogsColumnDefs';
import { getLogOther } from '../../../helpers';
import ChannelGroupRaceDetails from './components/ChannelGroupRaceDetails';

const getChannelGroupRaces = (record) =>
  getLogOther(record.other)?.admin_info?.channel_group_races;

const LogsTable = (logsData) => {
  const {
    logs,
    expandData,
    loading,
    activePage,
    pageSize,
    logCount,
    expandedRowKeys,
    mobileExpandedRowKeys,
    compactMode,
    visibleColumns,
    handlePageChange,
    handlePageSizeChange,
    handleRowExpand,
    handleMobileRowExpandChange,
    copyText,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
    hasExpandableRows,
    isAdminUser,
    billingDisplayMode,
    t,
    COLUMN_KEYS,
  } = logsData;

  // Get all columns
  const allColumns = useMemo(() => {
    return getLogsColumns({
      t,
      COLUMN_KEYS,
      copyText,
      showUserInfoFunc,
      openChannelAffinityUsageCacheModal,
      isAdminUser,
      billingDisplayMode,
    });
  }, [
    t,
    COLUMN_KEYS,
    copyText,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
    isAdminUser,
    billingDisplayMode,
  ]);

  // Filter columns based on visibility settings
  const getVisibleColumns = () => {
    return allColumns.filter((column) => visibleColumns[column.key]);
  };

  const visibleColumnsList = useMemo(() => {
    return getVisibleColumns();
  }, [visibleColumns, allColumns]);

  const tableColumns = useMemo(() => {
    return compactMode
      ? visibleColumnsList.map(({ fixed, ...rest }) => rest)
      : visibleColumnsList;
  }, [compactMode, visibleColumnsList]);

  const getExpandData = (record) => {
    const rowExpandData = expandData[record.key] || [];
    const raceDetails = getChannelGroupRaces(record);
    const details = [...rowExpandData];
    if (isAdminUser && Array.isArray(raceDetails) && raceDetails.length > 0) {
      details.unshift({
        key: t('渠道组竞速'),
        value: <ChannelGroupRaceDetails races={raceDetails} t={t} />,
      });
    }
    if (record.user_agent) {
      details.push({
        key: t('User-Agent'),
        value: (
          <span style={{ maxWidth: 600, wordBreak: 'break-all' }}>
            {record.user_agent}
          </span>
        ),
      });
    }
    return details;
  };

  const hasRowsWithDetails = () => {
    return (
      hasExpandableRows() ||
      logs.some(
        (record) =>
          Boolean(record.user_agent) ||
          (isAdminUser && getChannelGroupRaces(record)?.length > 0),
      )
    );
  };

  const expandRowRender = (record) => {
    return <Descriptions data={getExpandData(record)} />;
  };

  return (
    <CardTable
      columns={tableColumns}
      {...(hasRowsWithDetails() && {
        expandedRowRender: expandRowRender,
        expandRowByClick: true,
        expandedRowKeys,
        onExpand: handleRowExpand,
        rowExpandable: (record) => getExpandData(record).length > 0,
      })}
      dataSource={logs}
      rowKey='key'
      loading={loading}
      scroll={compactMode ? undefined : { x: 'max-content' }}
      className='rounded-xl overflow-hidden'
      size='small'
      mobileExpandedRowKeys={mobileExpandedRowKeys}
      onMobileRowExpandChange={handleMobileRowExpandChange}
      empty={
        <Empty
          image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
          darkModeImage={
            <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
          }
          description={t('搜索无结果')}
          style={{ padding: 30 }}
        />
      }
      pagination={{
        currentPage: activePage,
        pageSize: pageSize,
        total: logCount,
        pageSizeOptions: [10, 20, 50, 100],
        showSizeChanger: true,
        onPageSizeChange: (size) => {
          handlePageSizeChange(size);
        },
        onPageChange: handlePageChange,
      }}
      hidePagination={true}
    />
  );
};

export default LogsTable;
