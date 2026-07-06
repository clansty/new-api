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

import { getLogOther } from '../../../helpers/log';

export function toPositiveTokenNumber(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }
  return Math.trunc(parsed);
}

export function getCacheWriteTokens(other) {
  const cacheWriteTokens = toPositiveTokenNumber(other?.cache_write_tokens);
  if (cacheWriteTokens > 0) {
    return cacheWriteTokens;
  }

  const cacheCreationTokens5m = toPositiveTokenNumber(
    other?.cache_creation_tokens_5m,
  );
  const cacheCreationTokens1h = toPositiveTokenNumber(
    other?.cache_creation_tokens_1h,
  );
  if (cacheCreationTokens5m > 0 || cacheCreationTokens1h > 0) {
    return cacheCreationTokens5m + cacheCreationTokens1h;
  }

  return toPositiveTokenNumber(other?.cache_creation_tokens);
}

export function getPromptCacheSummary(other) {
  if (!other || typeof other !== 'object') {
    return null;
  }

  const cacheReadTokens = toPositiveTokenNumber(other.cache_tokens);
  const cacheWriteTokens = getCacheWriteTokens(other);

  if (cacheReadTokens <= 0 && cacheWriteTokens <= 0) {
    return null;
  }

  return {
    cacheReadTokens,
    cacheWriteTokens,
  };
}

export function getUsageCacheHitRate(record) {
  const other = getLogOther(record?.other);
  const promptTokens = toPositiveTokenNumber(record?.prompt_tokens);
  const cacheSummary = getPromptCacheSummary(other);
  const cacheReadTokens = cacheSummary?.cacheReadTokens || 0;
  const cacheWriteTokens = cacheSummary?.cacheWriteTokens || 0;
  const totalInputTokens = promptTokens + cacheReadTokens + cacheWriteTokens;

  if (totalInputTokens <= 0) {
    return null;
  }

  return (cacheReadTokens / totalInputTokens) * 100;
}

export function formatCacheHitRate(rate) {
  if (rate === null || rate === undefined || !Number.isFinite(rate)) {
    return '-';
  }

  return `${rate.toFixed(2)}%`;
}
