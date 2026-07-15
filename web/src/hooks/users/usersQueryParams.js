export const buildUsersExtraParams = (
  { hideZeroQuota, hideFullQuota, hideDeleted, sortBy, sortOrder },
  includeHiddenFilters = true,
) => {
  const params = new URLSearchParams();
  if (includeHiddenFilters) {
    if (hideZeroQuota) params.set('hide_zero_quota', 'true');
    if (hideFullQuota) params.set('hide_full_quota', 'true');
    if (hideDeleted) params.set('hide_deleted', 'true');
  }
  if (sortBy) params.set('sort_by', sortBy);
  params.set('sort_order', sortOrder);
  return `&${params.toString()}`;
};
