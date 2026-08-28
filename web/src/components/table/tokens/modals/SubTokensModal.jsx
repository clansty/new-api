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

import React, { useEffect, useState } from 'react';
import {
  Button,
  Input,
  Modal,
  Space,
  Table,
  Tag,
  Toast,
  Typography,
} from '@douyinfe/semi-ui';
import { IconCopy, IconKey, IconDelete } from '@douyinfe/semi-icons';
import { copy, timestamp2string } from '../../../../helpers';

const SubTokensModal = ({
  visible,
  parent,
  tokens,
  loading,
  onCancel,
  onCreate,
  onDelete,
  t,
}) => {
  const [name, setName] = useState('');
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState({});
  const [newKey, setNewKey] = useState('');

  useEffect(() => {
    if (!visible) {
      setName('');
      setNewKey('');
      setDeleting({});
    }
  }, [visible]);

  const create = async () => {
    const trimmedName = name.trim();
    if (!trimmedName) {
      Toast.error(t('请输入名称'));
      return;
    }
    setCreating(true);
    try {
      const data = await onCreate(trimmedName);
      setName('');
      setNewKey(data.key || '');
    } catch (error) {
      Toast.error(error.message || t('创建失败'));
    } finally {
      setCreating(false);
    }
  };

  const copyKey = async () => {
    if (newKey && (await copy(newKey))) {
      Toast.success(t('已复制到剪贴板！'));
    }
  };

  const columns = [
    { title: t('名称'), dataIndex: 'name' },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (status) => (
        <Tag color={status === 1 ? 'green' : 'red'} shape='circle'>
          {status === 1 ? t('已启用') : t('已禁用')}
        </Tag>
      ),
    },
    {
      title: t('过期时间'),
      dataIndex: 'expired_time',
      render: (value) =>
        value === -1 ? t('永不过期') : timestamp2string(value),
    },
    {
      title: t('操作'),
      render: (_, record) => (
        <Button
          type='danger'
          theme='borderless'
          size='small'
          icon={<IconDelete />}
          loading={deleting[record.id]}
          aria-label={t('删除子令牌')}
          onClick={() => {
            Modal.confirm({
              title: t('确定是否要删除此令牌？'),
              content: t('此修改将不可逆'),
              onOk: async () => {
                setDeleting((prev) => ({ ...prev, [record.id]: true }));
                try {
                  await onDelete(record.id);
                } finally {
                  setDeleting((prev) => {
                    const next = { ...prev };
                    delete next[record.id];
                    return next;
                  });
                }
              },
            });
          }}
        />
      ),
    },
  ];

  return (
    <Modal
      visible={visible}
      title={
        <Space>
          <IconKey />
          <span>{t('管理子令牌')}</span>
        </Space>
      }
      onCancel={onCancel}
      footer={null}
      width='min(720px, calc(100vw - 32px))'
    >
      <Space vertical align='start' className='w-full'>
        <Typography.Text type='tertiary'>
          {t('母令牌')}: {parent?.name || ''}
        </Typography.Text>
        <Space wrap className='w-full'>
          <Input
            value={name}
            placeholder={t('请输入子令牌名称')}
            maxLength={50}
            onChange={setName}
            onEnterPress={create}
            style={{ flex: 1, minWidth: 180 }}
          />
          <Button
            theme='solid'
            type='primary'
            loading={creating}
            onClick={create}
          >
            {t('创建子令牌')}
          </Button>
        </Space>
        {newKey && (
          <Space className='w-full' align='center'>
            <Input readOnly value={newKey} className='flex-1' />
            <Button
              theme='borderless'
              icon={<IconCopy />}
              aria-label={t('复制子令牌')}
              onClick={copyKey}
            />
          </Space>
        )}
        <Table
          rowKey='id'
          columns={columns}
          dataSource={tokens}
          loading={loading}
          pagination={false}
          scroll={{ x: 560 }}
          empty={t('暂无子令牌')}
          className='w-full'
        />
      </Space>
    </Modal>
  );
};

export default SubTokensModal;
