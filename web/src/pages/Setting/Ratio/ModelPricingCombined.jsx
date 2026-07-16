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

import React, { useState } from 'react';
import {
  Button,
  Popconfirm,
  Radio,
  RadioGroup,
  Space,
} from '@douyinfe/semi-ui';
import { IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import ModelPricingEditor from './components/ModelPricingEditor';
import ModelRatioSettings from './ModelRatioSettings';
import { API, showError, showSuccess } from '../../../helpers';

export default function ModelPricingCombined({ options, refresh }) {
  const { t } = useTranslation();
  const [editMode, setEditMode] = useState('visual');
  const [cleaning, setCleaning] = useState(false);

  const clearUncoveredPrices = async () => {
    setCleaning(true);
    try {
      const res = await API.post('/api/option/clear_uncovered_ratio');
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(res.data.message);
      await refresh();
    } catch (error) {
      showError(error);
    } finally {
      setCleaning(false);
    }
  };

  return (
    <div>
      <Space wrap style={{ marginTop: 12, marginBottom: 16 }}>
        <RadioGroup
          type='button'
          size='small'
          value={editMode}
          onChange={(e) => setEditMode(e.target.value)}
        >
          <Radio value='visual'>{t('可视化编辑')}</Radio>
          <Radio value='manual'>{t('手动编辑')}</Radio>
        </RadioGroup>
        <Popconfirm
          title={t('确定清空未覆盖的价格吗？')}
          content={t(
            '将删除所有未被任何现有渠道（含已禁用）声明的模型的价格与倍率配置，此操作不可逆',
          )}
          okType={'danger'}
          position='top'
          onConfirm={clearUncoveredPrices}
        >
          <Button type='danger' icon={<IconDelete />} loading={cleaning}>
            {t('删除无渠道模型价格')}
          </Button>
        </Popconfirm>
      </Space>
      {editMode === 'visual' ? (
        <ModelPricingEditor options={options} refresh={refresh} />
      ) : (
        <ModelRatioSettings options={options} refresh={refresh} />
      )}
    </div>
  );
}
