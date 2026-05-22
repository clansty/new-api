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

import React, { useContext, useState } from 'react';
import { Avatar } from '@douyinfe/semi-ui';
import { StatusContext } from '../../context/Status';
import {
  buildAvatarUrl,
  isAvatarFailed,
  markAvatarFailed,
  stringToColor,
} from '../../helpers';

const getInitial = (name) => {
  const s = String(name || '');
  return s.length > 0 ? s.slice(0, 1).toUpperCase() : '?';
};

const UserAvatar = ({
  oidcId,
  username,
  size = 'extra-small',
  className,
  style,
  onClick,
  children,
  ...rest
}) => {
  const [statusState] = useContext(StatusContext);
  const template = statusState?.status?.oidc_picture_endpoint || '';
  const url = buildAvatarUrl(oidcId, template);
  const [, forceRender] = useState(0);

  const usable = url && !isAvatarFailed(url);
  const fallback = children !== undefined ? children : getInitial(username);

  return (
    <Avatar
      size={size}
      color={stringToColor(String(username || ''))}
      src={usable ? url : undefined}
      alt={String(username || '')}
      onError={() => {
        if (!url || isAvatarFailed(url)) return;
        markAvatarFailed(url);
        // 触发重渲染，让下一次直接走字母 fallback，避免 Semi 内部 state 与缓存不一致
        forceRender((n) => n + 1);
      }}
      className={className}
      style={style}
      onClick={onClick}
      {...rest}
    >
      {fallback}
    </Avatar>
  );
};

export default UserAvatar;
