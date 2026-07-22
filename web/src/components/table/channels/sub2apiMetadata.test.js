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
import { formatSub2APIRate, resolveUpstreamRate } from './sub2apiMetadata';

describe('formatSub2APIRate', () => {
  test('preserves a free zero multiplier', () => {
    expect(formatSub2APIRate(0)).toBe('0x');
  });

  test('trims insignificant decimal places', () => {
    expect(formatSub2APIRate(0.45)).toBe('0.45x');
    expect(formatSub2APIRate(1.234567)).toBe('1.2346x');
  });

  test('hides missing or invalid values', () => {
    expect(formatSub2APIRate(null)).toBeNull();
    expect(formatSub2APIRate('invalid')).toBeNull();
  });
});

describe('resolveUpstreamRate', () => {
  test('prefers the automatically queried rate', () => {
    expect(resolveUpstreamRate(0.45, 0.6)).toEqual({
      rate: 0.45,
      source: 'automatic',
    });
  });

  test('uses the manual rate only when no automatic rate exists', () => {
    expect(resolveUpstreamRate(null, 0.6)).toEqual({
      rate: 0.6,
      source: 'manual',
    });
  });

  test('returns no rate when neither source is configured', () => {
    expect(resolveUpstreamRate(undefined, null)).toBeNull();
  });
});
