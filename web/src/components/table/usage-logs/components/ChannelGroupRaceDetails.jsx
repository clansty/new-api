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

import React from 'react';
import { Typography } from '@douyinfe/semi-ui';
import { CheckCircle2 } from 'lucide-react';

const memberLabel = (member) =>
  `#${member.member_id} ${member.member_name || ''}`.trim();

const ChannelGroupRaceDetails = ({ races, t }) => {
  if (!Array.isArray(races) || races.length === 0) {
    return null;
  }

  return (
    <div style={{ display: 'grid', gap: 12, maxWidth: 600 }}>
      {races.map((race, raceIndex) => {
        const members = Array.isArray(race.members) ? race.members : [];
        const winnerId = race.winner?.member_id;
        return (
          <div
            key={`${race.channel_id}-${raceIndex}`}
            style={{ display: 'grid', gap: 10, minWidth: 0 }}
          >
            <div style={{ display: 'grid', gap: 4, minWidth: 0 }}>
              <Typography.Text type='tertiary'>{t('渠道组')}</Typography.Text>
              <Typography.Text style={{ overflowWrap: 'anywhere' }}>
                {`#${race.channel_id} ${race.channel_name || ''}`.trim()}
              </Typography.Text>
            </div>
            <div style={{ display: 'grid', gap: 4, minWidth: 0 }}>
              <Typography.Text type='tertiary'>{t('尝试成员')}</Typography.Text>
              <div style={{ display: 'grid', gap: 4, minWidth: 0 }}>
                {members.map((member, memberIndex) => {
                  const isWinner = member.member_id === winnerId;
                  return (
                    <div
                      key={`${member.member_id}-${memberIndex}`}
                      style={{ display: 'flex', alignItems: 'center', gap: 6 }}
                    >
                      {isWinner && (
                        <CheckCircle2
                          size={14}
                          color='var(--semi-color-success)'
                          style={{ flex: '0 0 auto' }}
                        />
                      )}
                      <Typography.Text
                        style={{ minWidth: 0, overflowWrap: 'anywhere' }}
                      >
                        {memberLabel(member)}
                      </Typography.Text>
                    </div>
                  );
                })}
              </div>
            </div>
            <div style={{ display: 'grid', gap: 4, minWidth: 0 }}>
              <Typography.Text type='tertiary'>{t('最终选择')}</Typography.Text>
              {race.winner ? (
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <CheckCircle2
                    size={14}
                    color='var(--semi-color-success)'
                    style={{ flex: '0 0 auto' }}
                  />
                  <Typography.Text
                    style={{ minWidth: 0, overflowWrap: 'anywhere' }}
                  >
                    {memberLabel(race.winner)}
                  </Typography.Text>
                </div>
              ) : (
                <Typography.Text type='tertiary'>
                  {t('未选出胜者')}
                </Typography.Text>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
};

export default ChannelGroupRaceDetails;
