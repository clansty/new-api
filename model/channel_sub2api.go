package model

import "github.com/QuantumNous/new-api/common"

type ChannelSub2APIState struct {
	Username             string
	Password             string
	AccessToken          string
	RefreshToken         string
	AccessTokenExpiresAt int64
	RateMultiplier       *float64
	GroupName            string
	GroupDescription     string
}

func (channel *Channel) Sub2APIState() ChannelSub2APIState {
	return ChannelSub2APIState{
		Username:             channel.Sub2APIUsername,
		Password:             channel.Sub2APIPassword,
		AccessToken:          channel.Sub2APIAccessToken,
		RefreshToken:         channel.Sub2APIRefreshToken,
		AccessTokenExpiresAt: channel.Sub2APIAccessTokenExpiresAt,
		RateMultiplier:       channel.UpstreamRateMultiplier,
		GroupName:            channel.UpstreamGroupName,
		GroupDescription:     channel.UpstreamGroupDescription,
	}
}

func (channel *Channel) ApplySub2APIState(state ChannelSub2APIState) {
	channel.Sub2APIUsername = state.Username
	channel.Sub2APIPassword = state.Password
	channel.Sub2APIAccessToken = state.AccessToken
	channel.Sub2APIRefreshToken = state.RefreshToken
	channel.Sub2APIAccessTokenExpiresAt = state.AccessTokenExpiresAt
	channel.UpstreamRateMultiplier = state.RateMultiplier
	channel.UpstreamGroupName = state.GroupName
	channel.UpstreamGroupDescription = state.GroupDescription
}

func (channel *Channel) SaveSub2APIState(state ChannelSub2APIState) error {
	err := DB.Model(channel).Updates(map[string]any{
		"sub2api_username":                state.Username,
		"sub2api_password":                state.Password,
		"sub2api_access_token":            state.AccessToken,
		"sub2api_refresh_token":           state.RefreshToken,
		"sub2api_access_token_expires_at": state.AccessTokenExpiresAt,
		"upstream_rate_multiplier":        state.RateMultiplier,
		"upstream_group_name":             state.GroupName,
		"upstream_group_description":      state.GroupDescription,
	}).Error
	if err != nil {
		return err
	}
	channel.ApplySub2APIState(state)
	return nil
}

func (channel *Channel) SaveSub2APITokens(state ChannelSub2APIState) error {
	err := DB.Model(channel).Updates(map[string]any{
		"sub2api_access_token":            state.AccessToken,
		"sub2api_refresh_token":           state.RefreshToken,
		"sub2api_access_token_expires_at": state.AccessTokenExpiresAt,
	}).Error
	if err != nil {
		return err
	}
	channel.Sub2APIAccessToken = state.AccessToken
	channel.Sub2APIRefreshToken = state.RefreshToken
	channel.Sub2APIAccessTokenExpiresAt = state.AccessTokenExpiresAt
	return nil
}

func (channel *Channel) SaveSub2APIBalance(balance float64, rateMultiplier *float64, groupName, groupDescription string) error {
	now := common.GetTimestamp()
	err := DB.Model(channel).Updates(map[string]any{
		"balance_updated_time":       now,
		"balance":                    balance,
		"upstream_rate_multiplier":   rateMultiplier,
		"upstream_group_name":        groupName,
		"upstream_group_description": groupDescription,
	}).Error
	if err != nil {
		return err
	}
	channel.BalanceUpdatedTime = now
	channel.Balance = balance
	channel.UpstreamRateMultiplier = rateMultiplier
	channel.UpstreamGroupName = groupName
	channel.UpstreamGroupDescription = groupDescription
	return nil
}
