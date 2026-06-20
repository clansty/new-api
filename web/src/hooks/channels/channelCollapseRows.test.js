import { describe, expect, test } from 'bun:test';
import {
  COLLAPSED_CHANNELS_ROW_KEY,
  buildChannelRows,
} from './channelCollapseRows';

const t = (text) => text;

describe('buildChannelRows', () => {
  test('hides collapsed channels in normal mode and adds a summary entry', () => {
    const rows = buildChannelRows(
      [{ id: 10, key: '10', name: 'visible', status: 1, collapsed: false }],
      false,
      t,
      [
        {
          id: 20,
          key: '20',
          name: 'folded-enabled',
          status: 1,
          collapsed: true,
          inflight_count: 2,
        },
        {
          id: 30,
          key: '30',
          name: 'folded-auto-disabled',
          status: 3,
          collapsed: true,
          inflight_count: 3,
        },
      ],
    );

    expect(rows).toHaveLength(2);
    expect(rows[0].key).toBe(COLLAPSED_CHANNELS_ROW_KEY);
    expect(rows[0].collapsed_enabled_count).toBe(1);
    expect(rows[0].collapsed_auto_disabled_count).toBe(1);
    expect(rows[0].inflight_count).toBe(5);
    expect(rows[0].children.map((channel) => channel.id)).toEqual([20, 30]);
    expect(rows[1].id).toBe(10);
  });

  test('does not add a summary entry when no channels are collapsed', () => {
    const rows = buildChannelRows(
      [{ id: 10, key: '10', name: 'visible', status: 1, collapsed: false }],
      false,
      t,
      [],
    );

    expect(rows).toHaveLength(1);
    expect(rows[0].id).toBe(10);
  });

  test('keeps tag aggregation behavior independent from collapse state', () => {
    const rows = buildChannelRows(
      [
        {
          id: 10,
          key: '10',
          tag: 'alpha',
          group: 'default',
          used_quota: 10,
          inflight_count: 2,
          response_time: 100,
          priority: 1,
          weight: 1,
          status: 1,
          collapsed: true,
        },
      ],
      true,
      t,
      [],
    );

    expect(rows).toHaveLength(1);
    expect(rows[0].key).toBe('alpha');
    expect(rows[0].inflight_count).toBe(2);
    expect(rows[0].children.map((channel) => channel.id)).toEqual([10]);
  });
});
