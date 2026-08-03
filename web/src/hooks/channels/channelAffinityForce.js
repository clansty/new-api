export const indexChannelAffinityForces = (statuses) => {
  if (!Array.isArray(statuses)) return {};
  return statuses.reduce((indexed, status) => {
    const channelId = Number(status?.channel_id);
    if (!Number.isInteger(channelId) || channelId <= 0) return indexed;
    indexed[channelId] = status;
    return indexed;
  }, {});
};

export const formatChannelAffinityForceRemaining = (
  status,
  nowUnix = Math.floor(Date.now() / 1000),
) => {
  const forceUntil = Number(status?.force_until);
  if (!Number.isFinite(forceUntil)) return null;
  const remaining = Math.max(0, Math.ceil(forceUntil - nowUnix));
  if (remaining <= 0) return null;
  const minutes = Math.floor(remaining / 60);
  const seconds = remaining % 60;
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
};
