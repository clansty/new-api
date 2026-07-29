package model

import "github.com/QuantumNous/new-api/common"

type ChannelSub2APIState struct {
	Username               string
	Password               string
	AccessToken            string
	RefreshToken           string
	AccessTokenExpiresAt   int64
	RateMultiplier         *float64
	DeclaredRateMultiplier *float64
	LoginRateMultiplier    *float64
	GroupName              string
	GroupDescription       string
}

type Sub2APIBalanceSnapshot struct {
	Balance                float64
	DeclaredRateMultiplier *float64
	LoginRateMultiplier    *float64
	GroupName              string
	GroupDescription       string
}

func (snapshot Sub2APIBalanceSnapshot) EffectiveRateMultiplier() *float64 {
	if snapshot.DeclaredRateMultiplier != nil {
		return snapshot.DeclaredRateMultiplier
	}
	return snapshot.LoginRateMultiplier
}

func (channel *Channel) Sub2APIState() ChannelSub2APIState {
	return ChannelSub2APIState{
		Username:               channel.Sub2APIUsername,
		Password:               channel.Sub2APIPassword,
		AccessToken:            channel.Sub2APIAccessToken,
		RefreshToken:           channel.Sub2APIRefreshToken,
		AccessTokenExpiresAt:   channel.Sub2APIAccessTokenExpiresAt,
		RateMultiplier:         channel.UpstreamRateMultiplier,
		DeclaredRateMultiplier: channel.UpstreamDeclaredRateMultiplier,
		LoginRateMultiplier:    channel.UpstreamLoginRateMultiplier,
		GroupName:              channel.UpstreamGroupName,
		GroupDescription:       channel.UpstreamGroupDescription,
	}
}

func (channel *Channel) ApplySub2APIState(state ChannelSub2APIState) {
	channel.Sub2APIUsername = state.Username
	channel.Sub2APIPassword = state.Password
	channel.Sub2APIAccessToken = state.AccessToken
	channel.Sub2APIRefreshToken = state.RefreshToken
	channel.Sub2APIAccessTokenExpiresAt = state.AccessTokenExpiresAt
	channel.UpstreamRateMultiplier = state.RateMultiplier
	channel.UpstreamDeclaredRateMultiplier = state.DeclaredRateMultiplier
	channel.UpstreamLoginRateMultiplier = state.LoginRateMultiplier
	channel.UpstreamGroupName = state.GroupName
	channel.UpstreamGroupDescription = state.GroupDescription
}

func (channel *Channel) SaveSub2APIState(state ChannelSub2APIState) error {
	state.AccessToken = ""
	state.RefreshToken = ""
	state.AccessTokenExpiresAt = 0
	err := DB.Model(channel).Updates(map[string]any{
		"sub2api_username":                  state.Username,
		"sub2api_password":                  state.Password,
		"sub2api_access_token":              state.AccessToken,
		"sub2api_refresh_token":             state.RefreshToken,
		"sub2api_access_token_expires_at":   state.AccessTokenExpiresAt,
		"upstream_rate_multiplier":          state.RateMultiplier,
		"upstream_declared_rate_multiplier": state.DeclaredRateMultiplier,
		"upstream_login_rate_multiplier":    state.LoginRateMultiplier,
		"upstream_group_name":               state.GroupName,
		"upstream_group_description":        state.GroupDescription,
	}).Error
	if err != nil {
		return err
	}
	channel.ApplySub2APIState(state)
	return nil
}

func (channel *Channel) SaveSub2APIBalance(snapshot Sub2APIBalanceSnapshot) error {
	now := common.GetTimestamp()
	err := DB.Model(channel).Updates(map[string]any{
		"balance_updated_time":              now,
		"balance":                           snapshot.Balance,
		"upstream_rate_multiplier":          snapshot.EffectiveRateMultiplier(),
		"upstream_declared_rate_multiplier": snapshot.DeclaredRateMultiplier,
		"upstream_login_rate_multiplier":    snapshot.LoginRateMultiplier,
		"upstream_group_name":               snapshot.GroupName,
		"upstream_group_description":        snapshot.GroupDescription,
	}).Error
	if err != nil {
		return err
	}
	channel.BalanceUpdatedTime = now
	channel.Balance = snapshot.Balance
	channel.UpstreamRateMultiplier = snapshot.EffectiveRateMultiplier()
	channel.UpstreamDeclaredRateMultiplier = snapshot.DeclaredRateMultiplier
	channel.UpstreamLoginRateMultiplier = snapshot.LoginRateMultiplier
	channel.UpstreamGroupName = snapshot.GroupName
	channel.UpstreamGroupDescription = snapshot.GroupDescription
	return nil
}
