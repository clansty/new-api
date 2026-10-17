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

import React, { useEffect, useMemo, useState } from 'react';
import { Modal, Select, Typography } from '@douyinfe/semi-ui';
import { API, showError } from '../../../../helpers';

const normalizeModelName = (model) => String(model ?? '').trim();

const BatchModelModal = ({
  showBatchModelModal,
  setShowBatchModelModal,
  batchAddModels,
  setBatchAddModels,
  batchRemoveModels,
  setBatchRemoveModels,
  batchUpdateChannelModels,
  selectedChannels,
  t,
}) => {
  const [modelOptions, setModelOptions] = useState([]);

  // 删除候选项来自所选渠道已有的模型，方便管理员直接勾选
  const channelModelOptions = useMemo(() => {
    const modelSet = new Set();
    const collect = (channel) => {
      String(channel?.models || '')
        .split(',')
        .map(normalizeModelName)
        .filter(Boolean)
        .forEach((model) => modelSet.add(model));
    };
    (selectedChannels || []).forEach((channel) => {
      collect(channel);
      (channel.children || []).forEach(collect);
    });
    return Array.from(modelSet).sort();
  }, [selectedChannels]);

  useEffect(() => {
    if (!showBatchModelModal) {
      return;
    }
    let cancelled = false;
    const fetchModelOptions = async () => {
      try {
        const res = await API.get('/api/channel/models');
        const options = Array.from(
          new Set(
            (res?.data?.data || [])
              .map((model) => normalizeModelName(model?.id ?? model))
              .filter(Boolean),
          ),
        );
        if (!cancelled) {
          setModelOptions(options);
        }
      } catch (error) {
        if (!cancelled) {
          showError(error);
        }
      }
    };
    fetchModelOptions();
    return () => {
      cancelled = true;
    };
  }, [showBatchModelModal]);

  const addOptionList = useMemo(
    () => modelOptions.map((value) => ({ label: value, value })),
    [modelOptions],
  );
  const removeOptionList = useMemo(() => {
    const merged = new Set([...channelModelOptions, ...modelOptions]);
    return Array.from(merged)
      .sort()
      .map((value) => ({ label: value, value }));
  }, [channelModelOptions, modelOptions]);

  return (
    <Modal
      title={t('批量设置模型')}
      visible={showBatchModelModal}
      onOk={batchUpdateChannelModels}
      onCancel={() => setShowBatchModelModal(false)}
      maskClosable={false}
      centered={true}
      size='small'
      className='!rounded-lg'
    >
      <div className='mb-4'>
        <Typography.Text type='secondary'>
          {t(
            '增加模型会追加到渠道现有模型之后，删除模型只影响所选渠道中已存在的模型',
          )}
        </Typography.Text>
      </div>

      <div className='mb-2'>
        <Typography.Text strong>{t('增加模型')}</Typography.Text>
      </div>
      <Select
        multiple
        allowCreate
        filter
        placeholder={t('选择或输入要增加的模型')}
        value={batchAddModels}
        optionList={addOptionList}
        onChange={(value) => setBatchAddModels(value)}
        style={{ width: '100%' }}
      />

      <div className='mt-4 mb-2'>
        <Typography.Text strong>{t('删除模型')}</Typography.Text>
      </div>
      <Select
        multiple
        allowCreate
        filter
        placeholder={t('选择或输入要删除的模型')}
        value={batchRemoveModels}
        optionList={removeOptionList}
        onChange={(value) => setBatchRemoveModels(value)}
        style={{ width: '100%' }}
      />

      <div className='mt-4'>
        <Typography.Text type='secondary'>
          {t('已选择 ${count} 个渠道').replace(
            '${count}',
            selectedChannels?.length || 0,
          )}
        </Typography.Text>
      </div>
    </Modal>
  );
};

export default BatchModelModal;
