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
import { Checkbox, Input, Modal, Switch, Typography } from '@douyinfe/semi-ui';

const BatchTagModal = ({
  showBatchSetTag,
  setShowBatchSetTag,
  batchSetChannelTag,
  batchSetTagValue,
  setBatchSetTagValue,
  batchSetAutoRecoverEnabled,
  setBatchSetAutoRecoverEnabled,
  batchSetAutoRecoverValue,
  setBatchSetAutoRecoverValue,
  selectedChannels,
  t,
}) => {
  return (
    <Modal
      title={t('批量编辑渠道')}
      visible={showBatchSetTag}
      onOk={batchSetChannelTag}
      onCancel={() => setShowBatchSetTag(false)}
      maskClosable={false}
      centered={true}
      size='small'
      className='!rounded-lg'
    >
      <div className='mb-5'>
        <Typography.Text>{t('请输入要设置的标签名称')}</Typography.Text>
      </div>
      <Input
        placeholder={t('留空则不修改标签')}
        value={batchSetTagValue}
        onChange={(v) => setBatchSetTagValue(v)}
      />
      <div className='mt-4 flex items-center justify-between gap-3'>
        <Checkbox
          checked={batchSetAutoRecoverEnabled}
          onChange={(event) =>
            setBatchSetAutoRecoverEnabled(event.target.checked)
          }
        >
          {t('同步禁用后自动检测')}
        </Checkbox>
        <Switch
          size='small'
          checked={batchSetAutoRecoverValue}
          disabled={!batchSetAutoRecoverEnabled}
          checkedText={t('开')}
          uncheckedText={t('关')}
          onChange={(value) => setBatchSetAutoRecoverValue(value)}
        />
      </div>
      <div className='mt-2'>
        <Typography.Text type='secondary'>
          {t('开启后，自动禁用的渠道会纳入定时检测，测试成功后自动启用')}
        </Typography.Text>
      </div>
      <div className='mt-4'>
        <Typography.Text type='secondary'>
          {t('已选择 ${count} 个渠道').replace(
            '${count}',
            selectedChannels.length,
          )}
        </Typography.Text>
      </div>
    </Modal>
  );
};

export default BatchTagModal;
