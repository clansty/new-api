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
import {
  formatCacheHitRate,
  getPromptCacheSummary,
  getUsageCacheHitRate,
} from './cacheHitRate';

describe('usage log cache hit rate', () => {
  test('uses prompt tokens as total input for OpenAI semantics', () => {
    const rate = getUsageCacheHitRate({
      prompt_tokens: 100,
      other: JSON.stringify({
        cache_tokens: 50,
        cache_creation_tokens: 50,
      }),
    });

    expect(rate).toBe(50);
    expect(formatCacheHitRate(rate)).toBe('50.00%');
  });

  test('adds cache read and split cache creation tokens for Anthropic semantics', () => {
    const rate = getUsageCacheHitRate({
      prompt_tokens: 100,
      other: JSON.stringify({
        usage_semantic: 'anthropic',
        cache_tokens: 40,
        cache_creation_tokens: 999,
        cache_creation_tokens_5m: 20,
        cache_creation_tokens_1h: 20,
      }),
    });

    expect(rate).toBeCloseTo(22.2222, 4);
  });

  test('does not infer Anthropic semantics from Claude upstream display flag', () => {
    const rate = getUsageCacheHitRate({
      prompt_tokens: 180,
      other: JSON.stringify({
        claude: true,
        cache_tokens: 30,
        cache_creation_tokens: 50,
      }),
    });

    expect(rate).toBeCloseTo(16.6667, 4);
  });

  test('keeps explicit zero hit rate when input tokens exist', () => {
    const rate = getUsageCacheHitRate({
      prompt_tokens: 100,
      other: '{}',
    });

    expect(rate).toBe(0);
    expect(formatCacheHitRate(rate)).toBe('0.00%');
  });

  test('returns null when a row has no input tokens', () => {
    expect(
      getUsageCacheHitRate({
        prompt_tokens: 0,
        other: '{}',
      }),
    ).toBeNull();
  });

  test('summarizes cache write tokens with the same precedence as stats', () => {
    expect(
      getPromptCacheSummary({
        cache_tokens: 12,
        cache_write_tokens: 8,
        cache_creation_tokens: 99,
      }),
    ).toEqual({
      cacheReadTokens: 12,
      cacheWriteTokens: 8,
    });
  });
});
