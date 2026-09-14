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

import { useCallback, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  isAdmin,
  processModelsData,
  processGroupsData,
  processChannelsData,
  showError,
} from '../../helpers';
import { API_ENDPOINTS } from '../../constants/playground.constants';

export const useDataLoader = (
  userState,
  inputs,
  groups,
  channels,
  handleInputChange,
  setModels,
  setGroups,
  setChannels,
) => {
  const { t } = useTranslation();
  const isAdminUser = isAdmin();

  // 使用 ref 读取最新的 inputs，避免回调因 inputs 变化而频繁重建
  const inputsRef = useRef(inputs);
  inputsRef.current = inputs;

  // 记录上一次请求的模型筛选条件，避免重复请求
  const lastModelKeyRef = useRef(null);

  const applyModels = useCallback(
    (modelNames) => {
      const { modelOptions, selectedModel } = processModelsData(
        modelNames,
        inputsRef.current.model,
      );
      setModels(modelOptions);

      if (selectedModel !== inputsRef.current.model) {
        handleInputChange('model', selectedModel);
      }
    },
    [handleInputChange, setModels],
  );

  // 加载模型列表，可按分组或渠道筛选
  const loadModels = useCallback(
    async ({ group, channelId } = {}) => {
      const useChannel = channelId !== undefined && channelId !== null;
      const key = useChannel ? `channel:${channelId}` : `group:${group ?? ''}`;
      if (lastModelKeyRef.current === key) {
        return;
      }
      lastModelKeyRef.current = key;

      try {
        const params = {};
        if (useChannel) {
          params.channel_id = channelId;
        } else if (group !== undefined && group !== null && group !== '') {
          params.group = group;
        }

        const res = await API.get(
          API_ENDPOINTS.USER_MODELS,
          Object.keys(params).length > 0 ? { params } : undefined,
        );
        const { success, message, data } = res.data;

        if (success) {
          applyModels(data);
        } else {
          lastModelKeyRef.current = null;
          showError(t(message));
        }
      } catch (error) {
        lastModelKeyRef.current = null;
        showError(t('加载模型失败'));
      }
    },
    [applyModels, t],
  );

  const loadGroups = useCallback(async () => {
    try {
      const res = await API.get(API_ENDPOINTS.USER_GROUPS);
      const { success, message, data } = res.data;

      if (success) {
        const userGroup =
          userState?.user?.group ||
          JSON.parse(localStorage.getItem('user'))?.group;
        const groupOptions = processGroupsData(data, userGroup);
        setGroups(groupOptions);
      } else {
        showError(t(message));
      }
    } catch (error) {
      showError(t('加载分组失败'));
    }
  }, [userState, setGroups, t]);

  const loadChannels = useCallback(async () => {
    try {
      const res = await API.get(API_ENDPOINTS.PLAYGROUND_CHANNELS);
      const { success, message, data } = res.data;

      if (success) {
        const { channelOptions } = processChannelsData(
          data,
          inputsRef.current.channelId,
        );
        setChannels(channelOptions);
      } else {
        showError(t(message));
      }
    } catch (error) {
      showError(t('加载渠道失败'));
    }
  }, [setChannels, t]);

  // 首次加载：管理员加载渠道，普通用户加载分组
  useEffect(() => {
    if (!userState?.user) return;
    if (isAdminUser) {
      loadChannels();
    } else {
      loadGroups();
    }
  }, [userState?.user, isAdminUser, loadChannels, loadGroups]);

  // 普通用户：分组变化时按分组筛选模型
  useEffect(() => {
    if (!userState?.user || isAdminUser) return;
    if (groups.length === 0) return;

    const hasCurrentGroup = groups.some(
      (option) => option.value === inputs.group,
    );
    if (!hasCurrentGroup) {
      handleInputChange('group', groups[0].value);
      return;
    }
    loadModels({ group: inputs.group });
  }, [
    userState?.user,
    isAdminUser,
    groups,
    inputs.group,
    handleInputChange,
    loadModels,
  ]);

  // 管理员：渠道变化时按渠道筛选模型
  useEffect(() => {
    if (!userState?.user || !isAdminUser) return;
    if (channels.length === 0) return;

    const hasCurrentChannel = channels.some(
      (option) => option.value === inputs.channelId,
    );
    if (!hasCurrentChannel) {
      handleInputChange('channelId', channels[0].value);
      return;
    }
    loadModels({ channelId: inputs.channelId });
  }, [
    userState?.user,
    isAdminUser,
    channels,
    inputs.channelId,
    handleInputChange,
    loadModels,
  ]);

  return {
    loadModels,
    loadGroups,
    loadChannels,
  };
};
