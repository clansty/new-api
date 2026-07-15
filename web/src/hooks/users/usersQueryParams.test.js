import { describe, expect, test } from 'bun:test';
import { buildUsersExtraParams } from './usersQueryParams';

describe('buildUsersExtraParams', () => {
  test('搜索时忽略隐藏筛选并保留排序', () => {
    const params = buildUsersExtraParams(
      {
        hideZeroQuota: true,
        hideFullQuota: true,
        hideDeleted: true,
        sortBy: 'remaining_quota',
        sortOrder: 'asc',
      },
      false,
    );

    expect(params).toBe('&sort_by=remaining_quota&sort_order=asc');
  });
});
