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

import React, { useEffect, useRef, useState } from 'react';
import {
  Banner,
  Button,
  Form,
  Input,
  InputNumber,
  Select,
  Space,
  Spin,
  Table,
  Typography,
} from '@douyinfe/semi-ui';
import { IconDelete, IconPlus } from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const KEY_MODELS = 'model_retry_setting.models';

// 优先级选路策略，需与后端 operation_setting 常量保持一致
const STRATEGY_SAME = 'same';
const STRATEGY_NEXT = 'next';

// 将后端 map（模型名 -> 配置）解析为表格行
const mapToRows = (jsonString) => {
  let obj = {};
  try {
    obj = JSON.parse(jsonString || '{}') || {};
  } catch (e) {
    obj = {};
  }
  return Object.entries(obj).map(([model, cfg], index) => ({
    id: index,
    model,
    retry_times:
      cfg && cfg.retry_times !== undefined && cfg.retry_times !== null
        ? cfg.retry_times
        : '',
    priority_strategy:
      cfg && cfg.priority_strategy === STRATEGY_NEXT
        ? STRATEGY_NEXT
        : STRATEGY_SAME,
  }));
};

// 将表格行序列化回后端 map；空模型名跳过，未填重试次数则省略（沿用全局），same 策略省略
const rowsToMap = (rows) => {
  const map = {};
  (rows || []).forEach((row) => {
    const model = (row.model || '').trim();
    if (!model) return;
    const entry = {};
    if (
      row.retry_times !== '' &&
      row.retry_times !== null &&
      row.retry_times !== undefined
    ) {
      entry.retry_times = Number(row.retry_times);
    }
    if (row.priority_strategy === STRATEGY_NEXT) {
      entry.priority_strategy = STRATEGY_NEXT;
    }
    map[model] = entry;
  });
  return map;
};

export default function SettingsModelRetry(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState([]);
  const nextId = useRef(0);

  useEffect(() => {
    const parsed = mapToRows(props.options?.[KEY_MODELS]);
    nextId.current = parsed.length;
    setRows(parsed);
  }, [props.options]);

  const updateRow = (id, patch) => {
    setRows((prev) =>
      prev.map((row) => (row.id === id ? { ...row, ...patch } : row)),
    );
  };

  const addRow = () => {
    setRows((prev) => [
      ...prev,
      {
        id: nextId.current++,
        model: '',
        retry_times: '',
        priority_strategy: STRATEGY_SAME,
      },
    ]);
  };

  const deleteRow = (id) => {
    setRows((prev) => prev.filter((row) => row.id !== id));
  };

  const onSubmit = async () => {
    const map = rowsToMap(rows);
    setLoading(true);
    try {
      const res = await API.put('/api/option/', {
        key: KEY_MODELS,
        value: JSON.stringify(map),
      });
      if (res.data?.success) {
        showSuccess(t('保存成功'));
        props.refresh?.();
      } else {
        showError(res.data?.message || t('保存失败，请重试'));
      }
    } catch (e) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  const columns = [
    {
      title: t('模型名称'),
      dataIndex: 'model',
      render: (_, record) => (
        <Input
          placeholder={t('例如 gpt-4o、claude-3-5-sonnet')}
          value={record.model}
          aria-label={t('模型名称')}
          onChange={(value) => updateRow(record.id, { model: value })}
        />
      ),
    },
    {
      title: t('重试次数'),
      dataIndex: 'retry_times',
      width: 200,
      render: (_, record) => (
        <InputNumber
          style={{ width: '100%' }}
          min={0}
          placeholder={t('留空使用全局')}
          value={record.retry_times}
          aria-label={t('重试次数')}
          onChange={(value) =>
            updateRow(record.id, {
              retry_times: value === undefined || value === null ? '' : value,
            })
          }
        />
      ),
    },
    {
      title: t('失败后选路策略'),
      dataIndex: 'priority_strategy',
      width: 240,
      render: (_, record) => (
        <Select
          style={{ width: '100%' }}
          value={record.priority_strategy || STRATEGY_SAME}
          aria-label={t('失败后选路策略')}
          onChange={(value) =>
            updateRow(record.id, { priority_strategy: value })
          }
          optionList={[
            { label: t('同优先级优先'), value: STRATEGY_SAME },
            { label: t('直接下一优先级'), value: STRATEGY_NEXT },
          ]}
        />
      ),
    },
    {
      title: t('操作'),
      width: 90,
      render: (_, record) => (
        <Button
          icon={<IconDelete />}
          theme='borderless'
          type='danger'
          title={t('删除')}
          aria-label={t('删除')}
          onClick={() => deleteRow(record.id)}
        />
      ),
    },
  ];

  return (
    <Spin spinning={loading}>
      <Form style={{ marginBottom: 15 }}>
        <Form.Section text={t('模型级失败重试')}>
          <Banner
            fullMode={false}
            type='info'
            description={t(
              '为特定模型单独设置失败重试次数与失败后的渠道选路策略；未在此配置的模型使用全局“失败重试次数”。',
            )}
          />
          <div style={{ marginTop: 12 }}>
            <Text type='tertiary' size='small'>
              {t(
                '重试次数：留空则沿用全局设置，填 0 表示该模型失败后不重试。',
              )}
              <br />
              {t(
                '同优先级优先：当前优先级渠道全部失败后才降级到下一优先级（默认）。直接下一优先级：当前优先级任一渠道失败后立即尝试下一优先级渠道。',
              )}
            </Text>
          </div>

          <Space style={{ marginTop: 12, marginBottom: 10 }}>
            <Button icon={<IconPlus />} onClick={addRow}>
              {t('新增模型')}
            </Button>
            <Button theme='solid' onClick={onSubmit}>
              {t('保存')}
            </Button>
          </Space>

          <Table
            columns={columns}
            dataSource={rows}
            rowKey='id'
            pagination={false}
            size='small'
            empty={t('暂无模型级重试配置')}
          />
        </Form.Section>
      </Form>
    </Spin>
  );
}
