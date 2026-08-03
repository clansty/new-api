import { describe, expect, test } from 'bun:test';
import {
  formatChannelAffinityForceRemaining,
  indexChannelAffinityForces,
} from './channelAffinityForce';

describe('indexChannelAffinityForces', () => {
  test('indexes active force status by channel id', () => {
    const indexed = indexChannelAffinityForces([
      { channel_id: 12, force_until: 1000 },
      { channel_id: 34, force_until: 2000 },
    ]);

    expect(indexed[12].force_until).toBe(1000);
    expect(indexed[34].force_until).toBe(2000);
  });

  test('ignores invalid status entries', () => {
    expect(indexChannelAffinityForces([null, {}, { channel_id: 0 }])).toEqual(
      {},
    );
  });
});

describe('formatChannelAffinityForceRemaining', () => {
  test('formats remaining time as minutes and seconds', () => {
    expect(
      formatChannelAffinityForceRemaining({ force_until: 1300 }, 1000),
    ).toBe('05:00');
    expect(
      formatChannelAffinityForceRemaining({ force_until: 1061 }, 1000),
    ).toBe('01:01');
  });

  test('returns null after the force window expires', () => {
    expect(
      formatChannelAffinityForceRemaining({ force_until: 999 }, 1000),
    ).toBeNull();
  });
});
