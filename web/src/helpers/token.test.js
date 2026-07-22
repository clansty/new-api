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

import { describe, expect, test } from 'bun:test';
import { buildTokenGroupOptions } from './tokenGroupOptions';

describe('buildTokenGroupOptions', () => {
  test('空描述和重复描述不会覆盖分组名称或共享选择身份', () => {
    const options = buildTokenGroupOptions({
      empty: { desc: '', ratio: 1 },
      alpha: { desc: '相同描述', ratio: 1 },
      beta: { desc: '相同描述', ratio: 2 },
    });

    expect(options.map((option) => option.label)).toEqual([
      'empty',
      'alpha',
      'beta',
    ]);
    expect(options.map((option) => option.description)).toEqual([
      '',
      '相同描述',
      '相同描述',
    ]);
    expect(new Set(options.map((option) => option.label)).size).toBe(3);
  });
});
