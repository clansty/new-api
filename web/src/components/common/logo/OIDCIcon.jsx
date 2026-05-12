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

const OIDCIcon = ({ style, ...rest }) => {
  return (
    <img
      src='/rei.png'
      alt='OIDC'
      width={20}
      height={20}
      style={{ objectFit: 'contain', display: 'inline-block', ...style }}
      {...rest}
    />
  );
};

export default OIDCIcon;
