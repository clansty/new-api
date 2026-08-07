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
import { useTranslation } from 'react-i18next';
import {
  Button,
  Dropdown,
  Form,
  InputNumber,
  Modal,
  Popconfirm,
  Space,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconMore, IconPlus, IconSave } from '@douyinfe/semi-icons';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text } = Typography;

const statusTag = (status, t) => {
  if (status === 1) return <Tag color='green'>{t('已启用')}</Tag>;
  if (status === 2) return <Tag color='red'>{t('已禁用')}</Tag>;
  if (status === 3) return <Tag color='yellow'>{t('自动禁用')}</Tag>;
  return <Tag color='grey'>{t('未知状态')}</Tag>;
};

const ChannelGroupManageModal = ({ visible, channel, onCancel, onRefresh }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [members, setMembers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [parallelRequests, setParallelRequests] = useState(1);
  const [savingParallel, setSavingParallel] = useState(false);
  const [editingMember, setEditingMember] = useState(null);
  const [memberModalVisible, setMemberModalVisible] = useState(false);
  const [memberSubmitting, setMemberSubmitting] = useState(false);
  const [testingMemberId, setTestingMemberId] = useState(null);
  const [formApi, setFormApi] = useState(null);

  const loadMembers = async () => {
    if (!channel?.id) return;
    setLoading(true);
    try {
      const res = await API.get(`/api/channel/${channel.id}/members`);
      if (!res?.data?.success)
        throw new Error(res?.data?.message || t('加载失败'));
      setMembers(res.data.data || []);
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible || !channel?.id) return;
    setParallelRequests(Math.max(1, Number(channel.parallel_requests) || 1));
    loadMembers();
  }, [visible, channel?.id]);

  const openMemberModal = (member = null) => {
    setEditingMember(member);
    setMemberModalVisible(true);
  };

  const submitMember = async () => {
    const values = formApi?.getValues() || {};
    const payload = {
      name: String(values.name || '').trim(),
      key: String(values.key || '').trim(),
      base_url: String(values.base_url || '').trim() || null,
      proxy: String(values.proxy || '').trim() || null,
    };
    if (!payload.name || (!editingMember && !payload.key)) {
      showError(t('成员名称和密钥不能为空'));
      return;
    }
    setMemberSubmitting(true);
    try {
      const res = editingMember
        ? await API.put(
            `/api/channel/${channel.id}/members/${editingMember.id}`,
            payload,
          )
        : await API.post(`/api/channel/${channel.id}/members`, payload);
      if (!res?.data?.success)
        throw new Error(res?.data?.message || t('保存失败'));
      showSuccess(editingMember ? t('成员更新成功') : t('成员添加成功'));
      setMemberModalVisible(false);
      await loadMembers();
      onRefresh?.();
    } catch (error) {
      showError(error.message);
    } finally {
      setMemberSubmitting(false);
    }
  };

  const updateMemberStatus = async (member) => {
    const status = member.status === 1 ? 2 : 1;
    try {
      const res = await API.put(
        `/api/channel/${channel.id}/members/${member.id}/status`,
        { status },
      );
      if (!res?.data?.success)
        throw new Error(res?.data?.message || t('操作失败'));
      await loadMembers();
      onRefresh?.();
    } catch (error) {
      showError(error.message);
    }
  };

  const testMember = async (member) => {
    setTestingMemberId(member.id);
    try {
      const res = await API.get(
        `/api/channel/${channel.id}/members/${member.id}/test`,
      );
      if (!res?.data?.success)
        throw new Error(res?.data?.message || t('测试失败'));
      showSuccess(t('测试成功，耗时 {{time}} 秒', { time: res.data.time }));
      await loadMembers();
    } catch (error) {
      showError(error.message);
    } finally {
      setTestingMemberId(null);
    }
  };

  const deleteMember = async (member) => {
    try {
      const res = await API.delete(
        `/api/channel/${channel.id}/members/${member.id}`,
      );
      if (!res?.data?.success)
        throw new Error(res?.data?.message || t('删除失败'));
      showSuccess(t('成员已删除'));
      await loadMembers();
      onRefresh?.();
    } catch (error) {
      showError(error.message);
    }
  };

  const saveParallelRequests = async () => {
    setSavingParallel(true);
    try {
      const res = await API.put('/api/channel/', {
        id: channel.id,
        parallel_requests: parallelRequests,
      });
      if (!res?.data?.success)
        throw new Error(res?.data?.message || t('保存失败'));
      showSuccess(t('并行请求数已更新'));
      onRefresh?.();
    } catch (error) {
      showError(error.message);
    } finally {
      setSavingParallel(false);
    }
  };

  const columns = useMemo(() => {
    const memberColumns = [
      {
        title: t('成员'),
        dataIndex: 'name',
        width: isMobile ? 140 : undefined,
        render: (name, member) => (
          <Space vertical spacing={2} align='start'>
            <Text strong>{name}</Text>
            <Text code size='small'>
              {member.key_preview || '-'}
            </Text>
          </Space>
        ),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        width: isMobile ? 110 : undefined,
        render: (status, member) => (
          <Space vertical spacing={2} align='start'>
            {statusTag(status, t)}
            {member.disabled_reason ? (
              <Text
                type='tertiary'
                size='small'
                ellipsis={{ showTooltip: true }}
                style={{ maxWidth: 220 }}
              >
                {member.disabled_reason}
              </Text>
            ) : null}
          </Space>
        ),
      },
    ];

    if (!isMobile) {
      memberColumns.push({
        title: t('上游覆盖'),
        render: (_, member) => (
          <Space vertical spacing={2} align='start'>
            <Text size='small'>{member.base_url || t('继承渠道组')}</Text>
            <Text type='tertiary' size='small'>
              {member.proxy || t('继承代理设置')}
            </Text>
          </Space>
        ),
      });
      memberColumns.push({
        title: t('最近测试'),
        render: (_, member) => (
          <Space vertical spacing={2} align='start'>
            <Text>
              {member.response_time ? `${member.response_time} ms` : '-'}
            </Text>
            <Text type='tertiary' size='small'>
              {member.test_time ? timestamp2string(member.test_time) : '-'}
            </Text>
          </Space>
        ),
      });
    }

    memberColumns.push({
      title: isMobile ? null : t('操作'),
      width: isMobile ? 56 : 260,
      fixed: 'right',
      render: (_, member) =>
        isMobile ? (
          <Dropdown
            trigger='click'
            position='bottomRight'
            clickToHide
            menu={[
              {
                node: 'item',
                name: t('测试'),
                onClick: () => testMember(member),
              },
              {
                node: 'item',
                name: t('编辑'),
                onClick: () => openMemberModal(member),
              },
              {
                node: 'item',
                name: member.status === 1 ? t('禁用') : t('启用'),
                onClick: () => updateMemberStatus(member),
              },
              {
                node: 'item',
                name: t('删除'),
                type: 'danger',
                onClick: () => {
                  setTimeout(() => {
                    Modal.confirm({
                      title: t('确定要删除此成员吗？'),
                      width: isMobile ? 351 : 440,
                      style: { maxWidth: 'calc(100vw - 24px)' },
                      onOk: () => deleteMember(member),
                    });
                  }, 0);
                },
              },
            ]}
          >
            <Button
              size='small'
              type='tertiary'
              icon={<IconMore />}
              loading={testingMemberId === member.id}
              aria-label={t('成员操作')}
            />
          </Dropdown>
        ) : (
          <Space wrap>
            <Button
              size='small'
              loading={testingMemberId === member.id}
              onClick={() => testMember(member)}
            >
              {t('测试')}
            </Button>
            <Button
              size='small'
              type='tertiary'
              onClick={() => openMemberModal(member)}
            >
              {t('编辑')}
            </Button>
            <Button
              size='small'
              type={member.status === 1 ? 'danger' : 'primary'}
              onClick={() => updateMemberStatus(member)}
            >
              {member.status === 1 ? t('禁用') : t('启用')}
            </Button>
            <Popconfirm
              title={t('确定要删除此成员吗？')}
              onConfirm={() => deleteMember(member)}
            >
              <Button size='small' type='danger' theme='borderless'>
                {t('删除')}
              </Button>
            </Popconfirm>
          </Space>
        ),
    });

    return memberColumns;
  }, [isMobile, members, testingMemberId, t]);

  return (
    <>
      <Modal
        title={t('渠道组成员')}
        visible={visible}
        onCancel={onCancel}
        width={1060}
        style={{ maxWidth: 'calc(100vw - 24px)' }}
        footer={null}
      >
        <div className='flex flex-col gap-4'>
          <div className='flex flex-col md:flex-row md:items-end justify-between gap-3'>
            <Space align='end'>
              <div>
                <Text type='tertiary' size='small'>
                  {t('并行请求数')}
                </Text>
                <InputNumber
                  min={1}
                  max={4}
                  value={parallelRequests}
                  onChange={(value) => setParallelRequests(Number(value) || 1)}
                  style={{ width: 120, display: 'block', marginTop: 4 }}
                />
              </div>
              <Button
                icon={<IconSave />}
                loading={savingParallel}
                onClick={saveParallelRequests}
              >
                {t('保存')}
              </Button>
            </Space>
            <Button
              type='primary'
              icon={<IconPlus />}
              onClick={() => openMemberModal()}
            >
              {t('添加成员')}
            </Button>
          </div>
          <Table
            rowKey='id'
            columns={columns}
            dataSource={members}
            loading={loading}
            pagination={false}
            scroll={{ x: isMobile ? 306 : 900 }}
          />
        </div>
      </Modal>

      <Modal
        title={editingMember ? t('编辑成员') : t('添加成员')}
        visible={memberModalVisible}
        onCancel={() => setMemberModalVisible(false)}
        onOk={submitMember}
        confirmLoading={memberSubmitting}
        width={560}
        style={{ maxWidth: 'calc(100vw - 24px)' }}
      >
        <Form
          key={editingMember?.id || 'new'}
          getFormApi={setFormApi}
          labelPosition='top'
          initValues={{
            name: editingMember?.name || '',
            key: '',
            base_url: editingMember?.base_url || '',
            proxy: editingMember?.proxy || '',
          }}
        >
          <Form.Input
            field='name'
            label={t('成员名称')}
            rules={[{ required: true }]}
          />
          <Form.TextArea
            field='key'
            label={t('密钥')}
            placeholder={editingMember ? t('留空表示不修改') : t('请输入密钥')}
            autosize={{ minRows: 2, maxRows: 5 }}
          />
          <Form.Input
            field='base_url'
            label={t('API 地址覆盖')}
            placeholder={t('留空则继承渠道组')}
          />
          <Form.Input
            field='proxy'
            label={t('代理覆盖')}
            placeholder={t('留空则继承渠道组')}
          />
        </Form>
      </Modal>
    </>
  );
};

export default ChannelGroupManageModal;
