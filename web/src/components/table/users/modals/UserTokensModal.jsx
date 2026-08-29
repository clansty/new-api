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
  SideSheet,
  Space,
  Tag,
  Typography,
  Pagination,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { useTokensData } from '../../../../hooks/tokens/useTokensData';
import TokensTable from '../../tokens/TokensTable';
import TokensActions from '../../tokens/TokensActions';
import TokensFilters from '../../tokens/TokensFilters';
import EditTokenModal from '../../tokens/modals/EditTokenModal';
import SubTokensModal from '../../tokens/modals/SubTokensModal';

const { Title } = Typography;

const noop = () => {};

const UserTokensContent = ({ user }) => {
  const tokensData = useTokensData(noop, noop, { adminUserId: user.id });
  const { t } = tokensData;

  return (
    <div className='flex flex-col gap-3'>
      <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
        <TokensActions
          selectedKeys={tokensData.selectedKeys}
          setEditingToken={tokensData.setEditingToken}
          setShowEdit={tokensData.setShowEdit}
          batchCopyTokens={tokensData.batchCopyTokens}
          batchDeleteTokens={tokensData.batchDeleteTokens}
          t={t}
        />
        <TokensFilters
          formInitValues={tokensData.formInitValues}
          setFormApi={tokensData.setFormApi}
          searchTokens={tokensData.searchTokens}
          loading={tokensData.loading}
          searching={tokensData.searching}
          t={t}
        />
      </div>

      <TokensTable {...tokensData} />

      <div className='flex justify-end'>
        <Pagination
          total={tokensData.tokenCount}
          currentPage={tokensData.activePage}
          pageSize={tokensData.pageSize}
          showSizeChanger
          pageSizeOpts={[10, 20, 50, 100]}
          onPageChange={tokensData.handlePageChange}
          onPageSizeChange={tokensData.handlePageSizeChange}
        />
      </div>

      <EditTokenModal
        adminUserId={user.id}
        refresh={tokensData.refresh}
        editingToken={tokensData.editingToken}
        visiable={tokensData.showEdit}
        handleClose={tokensData.closeEdit}
      />

      <SubTokensModal
        visible={tokensData.showSubTokens}
        parent={tokensData.subTokenParent}
        tokens={tokensData.subTokens}
        loading={tokensData.subTokensLoading}
        onCancel={tokensData.closeSubTokens}
        onCreate={tokensData.createSubToken}
        onDelete={tokensData.deleteSubToken}
        t={t}
      />
    </div>
  );
};

const UserTokensModal = ({ visible, onCancel, user }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  return (
    <SideSheet
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('令牌')}
          </Tag>
          <Title heading={4} className='m-0'>
            {user ? `${user.username} ${t('的令牌')}` : t('用户令牌')}
          </Title>
        </Space>
      }
      visible={visible}
      onCancel={onCancel}
      width={isMobile ? '100%' : 1000}
      bodyStyle={{ overflow: 'auto', padding: 16 }}
    >
      {visible && user ? <UserTokensContent key={user.id} user={user} /> : null}
    </SideSheet>
  );
};

export default UserTokensModal;
