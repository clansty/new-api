export const COLLAPSED_CHANNELS_ROW_KEY = '__collapsed_channels__';

export const isCollapsedChannelsRow = (record) => {
  return record?.key === COLLAPSED_CHANNELS_ROW_KEY;
};

const createCollapsedChannelsRow = (channels, t) => {
  const enabledCount = channels.filter(
    (channel) => channel.status === 1,
  ).length;
  const autoDisabledCount = channels.filter(
    (channel) => channel.status === 3,
  ).length;

  return {
    key: COLLAPSED_CHANNELS_ROW_KEY,
    id: COLLAPSED_CHANNELS_ROW_KEY,
    name: t('已折叠渠道'),
    group: '',
    used_quota: channels.reduce(
      (total, channel) => total + (channel.used_quota || 0),
      0,
    ),
    inflight_count: channels.reduce(
      (total, channel) => total + (Number(channel.inflight_count) || 0),
      0,
    ),
    response_time: 0,
    priority: '',
    weight: '',
    status: 1,
    collapsed_entry: true,
    collapsed_enabled_count: enabledCount,
    collapsed_auto_disabled_count: autoDisabledCount,
    children: channels,
  };
};

export const buildChannelRows = (
  channels,
  enableTagMode,
  t,
  collapsedChannels = [],
) => {
  const rows = [];
  const channelTags = {};

  for (let i = 0; i < channels.length; i++) {
    const channel = channels[i];

    if (!enableTagMode) {
      if (!channel.collapsed) {
        rows.push(channel);
      }
      continue;
    }

    const tag = channel.tag ? channel.tag : '';
    let tagChannelRow = channelTags[tag];

    if (tagChannelRow === undefined) {
      tagChannelRow = {
        key: tag,
        id: tag,
        tag: tag,
        name: '标签：' + tag,
        group: '',
        used_quota: 0,
        response_time: 0,
        priority: -1,
        weight: -1,
        children: [],
      };
      channelTags[tag] = tagChannelRow;
      rows.push(tagChannelRow);
    }

    if (tagChannelRow.priority === -1) {
      tagChannelRow.priority = channel.priority;
    } else if (tagChannelRow.priority !== channel.priority) {
      tagChannelRow.priority = '';
    }

    if (tagChannelRow.weight === -1) {
      tagChannelRow.weight = channel.weight;
    } else if (tagChannelRow.weight !== channel.weight) {
      tagChannelRow.weight = '';
    }

    if (tagChannelRow.group === '') {
      tagChannelRow.group = channel.group;
    } else {
      const channelGroupsStr = channel.group;
      channelGroupsStr.split(',').forEach((item) => {
        if (tagChannelRow.group.indexOf(item) === -1) {
          tagChannelRow.group += ',' + item;
        }
      });
    }

    tagChannelRow.children.push(channel);
    if (channel.status === 1) {
      tagChannelRow.status = 1;
    }
    tagChannelRow.used_quota += channel.used_quota;
    tagChannelRow.inflight_count =
      (Number(tagChannelRow.inflight_count) || 0) +
      (Number(channel.inflight_count) || 0);
    tagChannelRow.response_time += channel.response_time;
    tagChannelRow.response_time = tagChannelRow.response_time / 2;
  }

  if (enableTagMode) {
    return rows;
  }

  if (collapsedChannels.length === 0) {
    return rows;
  }

  return [createCollapsedChannelsRow(collapsedChannels, t), ...rows];
};
