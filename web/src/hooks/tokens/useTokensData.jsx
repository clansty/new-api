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

import { useState, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal } from '@douyinfe/semi-ui';
import {
  API,
  copy,
  showError,
  showSuccess,
  encodeToBase64,
} from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useTableCompactMode } from '../common/useTableCompactMode';
import {
  fetchTokenKey as fetchTokenKeyById,
  fetchTokenKeysBatch,
  getServerAddress,
  encodeChannelConnectionString,
} from '../../helpers/token';

export const useTokensData = (
  openFluentNotification,
  openCCSwitchModal,
  options = {},
) => {
  const { t } = useTranslation();

  // adminUserId 不为空时切换到管理员令牌接口，操作指定用户的令牌
  const adminUserId = options.adminUserId ?? null;
  const tokenBasePath =
    adminUserId != null ? `/api/user/${adminUserId}/tokens` : '/api/token';
  const groupsPath =
    adminUserId != null
      ? `/api/user/${adminUserId}/groups`
      : '/api/user/self/groups';

  // Basic state
  const [tokens, setTokens] = useState([]);
  const [loading, setLoading] = useState(true);
  const [groupRatios, setGroupRatios] = useState({});
  const [activePage, setActivePage] = useState(1);
  const [tokenCount, setTokenCount] = useState(0);
  const [pageSize, setPageSize] = useState(
    () => parseInt(localStorage.getItem('page-size')) || ITEMS_PER_PAGE,
  );
  const [searching, setSearching] = useState(false);
  const [searchMode, setSearchMode] = useState(false); // 是否处于搜索结果视图

  // Selection state
  const [selectedKeys, setSelectedKeys] = useState([]);

  // Edit state
  const [showEdit, setShowEdit] = useState(false);
  const [editingToken, setEditingToken] = useState({
    id: undefined,
  });

  // UI state
  const [compactMode, setCompactMode] = useTableCompactMode('tokens');
  const [showKeys, setShowKeys] = useState({});
  const [resolvedTokenKeys, setResolvedTokenKeys] = useState({});
  const [loadingTokenKeys, setLoadingTokenKeys] = useState({});
  const keyRequestsRef = useRef({});
  const [subTokenParent, setSubTokenParent] = useState(null);
  const [subTokens, setSubTokens] = useState([]);
  const [subTokensLoading, setSubTokensLoading] = useState(false);
  const [showSubTokens, setShowSubTokens] = useState(false);

  // Form state
  const [formApi, setFormApi] = useState(null);
  const formInitValues = {
    searchKeyword: '',
    searchToken: '',
  };

  // Get form values helper function
  const getFormValues = () => {
    const formValues = formApi ? formApi.getValues() : {};
    return {
      searchKeyword: formValues.searchKeyword || '',
      searchToken: formValues.searchToken || '',
    };
  };

  // Close edit modal
  const closeEdit = () => {
    setShowEdit(false);
    setTimeout(() => {
      setEditingToken({
        id: undefined,
      });
    }, 500);
  };

  // Sync page data from API response
  const syncPageData = (payload) => {
    setTokens(payload.items || []);
    setTokenCount(payload.total || 0);
    setActivePage(payload.page || 1);
    setPageSize(payload.page_size || pageSize);
    setShowKeys({});
  };

  // Load tokens function
  const loadTokens = async (page = 1, size = pageSize) => {
    setLoading(true);
    setSearchMode(false);
    const res = await API.get(`${tokenBasePath}/?p=${page}&size=${size}`);
    const { success, message, data } = res.data;
    if (success) {
      syncPageData(data);
    } else {
      showError(message);
    }
    setLoading(false);
  };

  // Refresh function
  const refresh = async (page = activePage) => {
    await loadTokens(page);
    setSelectedKeys([]);
  };

  // Copy text function
  const copyText = async (text) => {
    if (await copy(text)) {
      showSuccess(t('已复制到剪贴板！'));
    } else {
      Modal.error({
        title: t('无法复制到剪贴板，请手动复制'),
        content: text,
        size: 'large',
      });
    }
  };

  const fetchTokenKey = async (tokenOrId, options = {}) => {
    const { suppressError = false } = options;
    const tokenId =
      typeof tokenOrId === 'object' ? tokenOrId?.id : Number(tokenOrId);

    if (!tokenId) {
      const error = new Error(t('令牌不存在'));
      if (!suppressError) {
        showError(error.message);
      }
      throw error;
    }

    if (resolvedTokenKeys[tokenId]) {
      return resolvedTokenKeys[tokenId];
    }

    if (keyRequestsRef.current[tokenId]) {
      return keyRequestsRef.current[tokenId];
    }

    const request = (async () => {
      setLoadingTokenKeys((prev) => ({ ...prev, [tokenId]: true }));
      try {
        const fullKey = await fetchTokenKeyById(tokenId, tokenBasePath);
        setResolvedTokenKeys((prev) => ({ ...prev, [tokenId]: fullKey }));
        return fullKey;
      } catch (error) {
        const normalizedError = new Error(
          error?.message || t('获取令牌密钥失败'),
        );
        if (!suppressError) {
          showError(normalizedError.message);
        }
        throw normalizedError;
      } finally {
        delete keyRequestsRef.current[tokenId];
        setLoadingTokenKeys((prev) => {
          const next = { ...prev };
          delete next[tokenId];
          return next;
        });
      }
    })();

    keyRequestsRef.current[tokenId] = request;
    return request;
  };

  const toggleTokenVisibility = async (record) => {
    const tokenId = record?.id;
    if (!tokenId) {
      return;
    }

    if (showKeys[tokenId]) {
      setShowKeys((prev) => ({ ...prev, [tokenId]: false }));
      return;
    }

    const fullKey = await fetchTokenKey(record);
    if (fullKey) {
      setShowKeys((prev) => ({ ...prev, [tokenId]: true }));
    }
  };

  const copyTokenKey = async (record) => {
    const fullKey = await fetchTokenKey(record);
    await copyText(`sk-${fullKey}`);
  };

  const copyTokenConnectionString = async (record) => {
    const fullKey = await fetchTokenKey(record);
    const serverUrl = getServerAddress();
    const connStr = encodeChannelConnectionString(`sk-${fullKey}`, serverUrl);
    await copyText(connStr);
  };

  const subTokenPath = (parentId) =>
    adminUserId != null
      ? `/api/user/${adminUserId}/tokens/${parentId}/subkeys`
      : `/api/token/${parentId}/subkeys`;

  const loadSubTokens = async (parentId) => {
    setSubTokensLoading(true);
    try {
      const res = await API.get(subTokenPath(parentId));
      if (res.data.success) {
        setSubTokens(res.data.data || []);
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error?.message || t('获取失败'));
    } finally {
      setSubTokensLoading(false);
    }
  };

  const openSubTokens = async (parent) => {
    setSubTokenParent(parent);
    setShowSubTokens(true);
    await loadSubTokens(parent.id);
  };

  const closeSubTokens = () => {
    setShowSubTokens(false);
    setSubTokenParent(null);
    setSubTokens([]);
  };

  const createSubToken = async (name) => {
    const res = await API.post(subTokenPath(subTokenParent.id), { name });
    if (!res.data.success) {
      throw new Error(res.data.message || t('创建失败'));
    }
    await loadSubTokens(subTokenParent.id);
    return res.data.data;
  };

  const deleteSubToken = async (subTokenId) => {
    const res = await API.delete(
      `${subTokenPath(subTokenParent.id)}/${subTokenId}`,
    );
    if (!res.data.success) {
      throw new Error(res.data.message || t('删除失败'));
    }
    await loadSubTokens(subTokenParent.id);
  };

  // Open link function for chat integrations
  const onOpenLink = async (type, url, record) => {
    const fullKey = await fetchTokenKey(record);
    if (url && url.startsWith('ccswitch')) {
      openCCSwitchModal(fullKey);
      return;
    }
    if (url && url.startsWith('fluent')) {
      openFluentNotification(fullKey);
      return;
    }
    let status = localStorage.getItem('status');
    let serverAddress = '';
    if (status) {
      status = JSON.parse(status);
      serverAddress = status.server_address;
    }
    if (serverAddress === '') {
      serverAddress = window.location.origin;
    }
    if (url.includes('{cherryConfig}') === true) {
      let cherryConfig = {
        id: 'new-api',
        baseUrl: serverAddress,
        apiKey: `sk-${fullKey}`,
      };
      let encodedConfig = encodeURIComponent(
        encodeToBase64(JSON.stringify(cherryConfig)),
      );
      url = url.replaceAll('{cherryConfig}', encodedConfig);
    } else if (url.includes('{aionuiConfig}') === true) {
      let aionuiConfig = {
        platform: 'new-api',
        baseUrl: serverAddress,
        apiKey: `sk-${fullKey}`,
      };
      let encodedConfig = encodeURIComponent(
        encodeToBase64(JSON.stringify(aionuiConfig)),
      );
      url = url.replaceAll('{aionuiConfig}', encodedConfig);
    } else {
      let encodedServerAddress = encodeURIComponent(serverAddress);
      url = url.replaceAll('{address}', encodedServerAddress);
      url = url.replaceAll('{key}', `sk-${fullKey}`);
    }

    window.open(url, '_blank');
  };

  // Manage token function (delete, enable, disable)
  const manageToken = async (id, action, record) => {
    setLoading(true);
    let data = { id };
    let res;
    switch (action) {
      case 'delete':
        res = await API.delete(`${tokenBasePath}/${id}`);
        break;
      case 'enable':
        data.status = 1;
        res = await API.put(`${tokenBasePath}/?status_only=true`, data);
        break;
      case 'disable':
        data.status = 2;
        res = await API.put(`${tokenBasePath}/?status_only=true`, data);
        break;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('操作成功完成！'));
      let token = res.data.data;
      let newTokens = [...tokens];
      if (action !== 'delete') {
        record.status = token.status;
      }
      setTokens(newTokens);
    } else {
      showError(message);
    }
    setLoading(false);
  };

  // Search tokens function
  const searchTokens = async (page = 1, size = pageSize) => {
    const normalizedPage = Number.isInteger(page) && page > 0 ? page : 1;
    const normalizedSize = Number.isInteger(size) && size > 0 ? size : pageSize;

    const { searchKeyword, searchToken } = getFormValues();
    if (searchKeyword === '' && searchToken === '') {
      setSearchMode(false);
      await loadTokens(1);
      return;
    }
    setSearching(true);
    const res = await API.get(
      `${tokenBasePath}/search?keyword=${encodeURIComponent(searchKeyword)}&token=${encodeURIComponent(searchToken)}&p=${normalizedPage}&size=${normalizedSize}`,
    );
    const { success, message, data } = res.data;
    if (success) {
      setSearchMode(true);
      syncPageData(data);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  // Sort tokens function
  const sortToken = (key) => {
    if (tokens.length === 0) return;
    setLoading(true);
    let sortedTokens = [...tokens];
    sortedTokens.sort((a, b) => {
      return ('' + a[key]).localeCompare(b[key]);
    });
    if (sortedTokens[0].id === tokens[0].id) {
      sortedTokens.reverse();
    }
    setTokens(sortedTokens);
    setLoading(false);
  };

  // Page handlers
  const handlePageChange = (page) => {
    if (searchMode) {
      searchTokens(page, pageSize).then();
    } else {
      loadTokens(page, pageSize).then();
    }
  };

  const handlePageSizeChange = async (size) => {
    localStorage.setItem('page-size', size + '');
    setPageSize(size);
    if (searchMode) {
      await searchTokens(1, size);
    } else {
      await loadTokens(1, size);
    }
  };

  // Row selection handlers
  const rowSelection = {
    onSelect: (record, selected) => {},
    onSelectAll: (selected, selectedRows) => {},
    onChange: (selectedRowKeys, selectedRows) => {
      setSelectedKeys(selectedRows);
    },
  };

  // Handle row styling
  const handleRow = (record, index) => {
    if (record.status !== 1) {
      return {
        style: {
          background: 'var(--semi-color-disabled-border)',
        },
      };
    } else {
      return {};
    }
  };

  // Batch delete tokens
  const batchDeleteTokens = async () => {
    if (selectedKeys.length === 0) {
      showError(t('请先选择要删除的令牌！'));
      return;
    }
    setLoading(true);
    try {
      const ids = selectedKeys.map((token) => token.id);
      const res = await API.post(`${tokenBasePath}/batch`, { ids });
      if (res?.data?.success) {
        const count = res.data.data || 0;
        showSuccess(t('已删除 {{count}} 个令牌！', { count }));
        await refresh();
        setTimeout(() => {
          if (tokens.length === 0 && activePage > 1) {
            refresh(activePage - 1);
          }
        }, 100);
      } else {
        showError(res?.data?.message || t('删除失败'));
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  // Batch copy tokens
  const batchCopyTokens = async (copyType) => {
    if (selectedKeys.length === 0) {
      showError(t('请至少选择一个令牌！'));
      return;
    }
    try {
      const ids = selectedKeys.map((token) => token.id);
      const keysMap = await fetchTokenKeysBatch(ids, tokenBasePath);

      setResolvedTokenKeys((prev) => ({ ...prev, ...keysMap }));

      let content = '';
      for (const token of selectedKeys) {
        const fullKey = keysMap[token.id];
        if (!fullKey) continue;
        if (copyType === 'name+key') {
          content += `${token.name}    sk-${fullKey}\n`;
        } else {
          content += `sk-${fullKey}\n`;
        }
      }
      await copyText(content);
    } catch (error) {
      showError(error?.message || t('复制令牌失败'));
    }
  };

  // Initialize data
  useEffect(() => {
    loadTokens(1)
      .then()
      .catch((reason) => {
        showError(reason);
      });
    API.get(groupsPath)
      .then((res) => {
        if (res.data.success && res.data.data) {
          const ratios = {};
          for (const [name, info] of Object.entries(res.data.data)) {
            ratios[name] = info.ratio;
          }
          setGroupRatios(ratios);
        }
      })
      .catch(() => {});
  }, [pageSize]);

  return {
    // Basic state
    tokens,
    loading,
    activePage,
    tokenCount,
    pageSize,
    searching,
    groupRatios,
    enableSubTokens: true,

    // Selection state
    selectedKeys,
    setSelectedKeys,

    // Edit state
    showEdit,
    setShowEdit,
    editingToken,
    setEditingToken,
    closeEdit,

    // UI state
    compactMode,
    setCompactMode,
    showKeys,
    setShowKeys,
    resolvedTokenKeys,
    loadingTokenKeys,
    subTokenParent,
    subTokens,
    subTokensLoading,
    showSubTokens,

    // Form state
    formApi,
    setFormApi,
    formInitValues,
    getFormValues,

    // Functions
    loadTokens,
    refresh,
    copyText,
    fetchTokenKey,
    toggleTokenVisibility,
    copyTokenKey,
    copyTokenConnectionString,
    onOpenLink,
    manageToken,
    searchTokens,
    sortToken,
    handlePageChange,
    handlePageSizeChange,
    rowSelection,
    handleRow,
    batchDeleteTokens,
    batchCopyTokens,
    syncPageData,
    openSubTokens,
    closeSubTokens,
    createSubToken,
    deleteSubToken,

    // Translation
    t,
  };
};
