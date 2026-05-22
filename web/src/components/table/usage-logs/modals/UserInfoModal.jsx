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
import { Modal, Badge, Typography } from '@douyinfe/semi-ui';
import {
  IconGithubLogo,
  IconMail,
  IconKey,
} from '@douyinfe/semi-icons';
import { SiTelegram, SiWechat, SiLinux, SiDiscord } from 'react-icons/si';
import {
  renderQuota,
  renderNumber,
  getOAuthProviderIcon,
} from '../../../../helpers';
import UserAvatar from '../../../common/UserAvatar';

const BINDING_DEFS = [
  { key: 'email', label: '邮箱', icon: <IconMail /> },
  { key: 'oidc_id', label: 'OIDC', icon: <IconKey /> },
  { key: 'github_id', label: 'GitHub', icon: <IconGithubLogo /> },
  { key: 'discord_id', label: 'Discord', icon: <SiDiscord size={14} /> },
  { key: 'telegram_id', label: 'Telegram', icon: <SiTelegram size={14} /> },
  { key: 'linux_do_id', label: 'LinuxDO', icon: <SiLinux size={14} /> },
  { key: 'wechat_id', label: '微信', icon: <SiWechat size={14} /> },
];

const UserInfoModal = ({
  showUserInfo,
  setShowUserInfoModal,
  userInfoData,
  t,
}) => {
  const infoItemStyle = {
    marginBottom: '16px',
  };

  const labelStyle = {
    display: 'flex',
    alignItems: 'center',
    marginBottom: '2px',
    fontSize: '12px',
    color: 'var(--semi-color-text-2)',
    gap: '6px',
  };

  const renderLabel = (text, type = 'tertiary') => (
    <div style={labelStyle}>
      <Badge dot type={type} />
      {text}
    </div>
  );

  const valueStyle = {
    fontSize: '14px',
    fontWeight: '600',
    color: 'var(--semi-color-text-0)',
  };

  const rowStyle = {
    display: 'flex',
    justifyContent: 'space-between',
    marginBottom: '16px',
    gap: '20px',
  };

  const colStyle = {
    flex: 1,
    minWidth: 0,
  };

  const builtInBindings = userInfoData
    ? BINDING_DEFS.filter((def) => userInfoData[def.key])
    : [];
  const customBindings = userInfoData?.oauth_bindings || [];
  const hasBindings = builtInBindings.length > 0 || customBindings.length > 0;

  const bindingTagStyle = {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 6,
    padding: '4px 10px',
    borderRadius: 999,
    background: 'var(--semi-color-fill-0)',
    color: 'var(--semi-color-text-0)',
    fontSize: 12,
    maxWidth: '100%',
  };

  return (
    <Modal
      title={t('用户信息')}
      visible={showUserInfo}
      onCancel={() => setShowUserInfoModal(false)}
      footer={null}
      centered
      closable
      maskClosable
      width={600}
    >
      {userInfoData && (
        <div style={{ padding: 20 }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 16,
              marginBottom: 24,
            }}
          >
            <UserAvatar
              size='large'
              oidcId={userInfoData.oidc_id}
              username={userInfoData.username}
            />
            <div style={{ minWidth: 0, flex: 1 }}>
              <Typography.Title heading={5} style={{ margin: 0 }} ellipsis>
                {userInfoData.display_name || userInfoData.username}
              </Typography.Title>
              <Typography.Text type='tertiary' size='small' ellipsis>
                @{userInfoData.username}
                {userInfoData.id ? `  ·  ID: ${userInfoData.id}` : ''}
              </Typography.Text>
            </div>
          </div>

          <div style={rowStyle}>
            <div style={colStyle}>
              {renderLabel(t('余额'), 'success')}
              <div style={valueStyle}>{renderQuota(userInfoData.quota)}</div>
            </div>
            <div style={colStyle}>
              {renderLabel(t('已用额度'), 'warning')}
              <div style={valueStyle}>
                {renderQuota(userInfoData.used_quota)}
              </div>
            </div>
          </div>

          <div style={rowStyle}>
            <div style={colStyle}>
              {renderLabel(t('请求次数'), 'warning')}
              <div style={valueStyle}>
                {renderNumber(userInfoData.request_count)}
              </div>
            </div>
            {userInfoData.group && (
              <div style={colStyle}>
                {renderLabel(t('用户组'), 'tertiary')}
                <div style={valueStyle}>{userInfoData.group}</div>
              </div>
            )}
          </div>

          {hasBindings && (
            <div style={infoItemStyle}>
              {renderLabel(t('绑定账号'), 'primary')}
              <div
                style={{
                  display: 'flex',
                  flexWrap: 'wrap',
                  gap: 8,
                  marginTop: 4,
                }}
              >
                {builtInBindings.map((def) => (
                  <span
                    key={def.key}
                    style={bindingTagStyle}
                    title={`${t(def.label)}: ${userInfoData[def.key]}`}
                  >
                    {def.icon}
                    <span style={{ fontWeight: 600 }}>{t(def.label)}</span>
                    <span
                      style={{
                        color: 'var(--semi-color-text-2)',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                        maxWidth: 220,
                      }}
                    >
                      {userInfoData[def.key]}
                    </span>
                  </span>
                ))}
                {customBindings.map((b) => (
                  <span
                    key={`custom-${b.provider_id}`}
                    style={bindingTagStyle}
                    title={`${b.provider_name}: ${b.provider_user_id}`}
                  >
                    {getOAuthProviderIcon(b.provider_icon, 16)}
                    <span style={{ fontWeight: 600 }}>{b.provider_name}</span>
                    <span
                      style={{
                        color: 'var(--semi-color-text-2)',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                        maxWidth: 220,
                      }}
                    >
                      {b.provider_user_id}
                    </span>
                  </span>
                ))}
              </div>
            </div>
          )}

          {(userInfoData.aff_code || userInfoData.aff_count !== undefined) && (
            <div style={rowStyle}>
              {userInfoData.aff_code && (
                <div style={colStyle}>
                  {renderLabel(t('邀请码'), 'tertiary')}
                  <div style={valueStyle}>{userInfoData.aff_code}</div>
                </div>
              )}
              {userInfoData.aff_count !== undefined && (
                <div style={colStyle}>
                  {renderLabel(t('邀请人数'), 'tertiary')}
                  <div style={valueStyle}>
                    {renderNumber(userInfoData.aff_count)}
                  </div>
                </div>
              )}
            </div>
          )}

          {userInfoData.aff_quota !== undefined &&
            userInfoData.aff_quota > 0 && (
              <div style={infoItemStyle}>
                {renderLabel(t('邀请获得额度'), 'success')}
                <div style={valueStyle}>
                  {renderQuota(userInfoData.aff_quota)}
                </div>
              </div>
            )}

          {userInfoData.remark && (
            <div style={{ marginBottom: 0 }}>
              {renderLabel(t('备注'), 'tertiary')}
              <div
                style={{
                  ...valueStyle,
                  wordBreak: 'break-all',
                  lineHeight: '1.4',
                }}
              >
                {userInfoData.remark}
              </div>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
};

export default UserInfoModal;
