package controller

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func resolveSub2APIAuthUpdate(incoming, origin *model.Channel, passwordInput *string) (model.ChannelSub2APIState, error) {
	if incoming.GetSetting().BalanceQueryMode != dto.BalanceQueryModeSub2API {
		return model.ChannelSub2APIState{}, nil
	}

	username := strings.TrimSpace(incoming.Sub2APIUsername)
	password := ""
	if passwordInput != nil {
		password = *passwordInput
	}
	if username == "" {
		if password != "" {
			return model.ChannelSub2APIState{}, errors.New("sub2api username is required when password is set")
		}
		return model.ChannelSub2APIState{}, nil
	}

	state := model.ChannelSub2APIState{Username: username, Password: password}
	if origin == nil {
		if password == "" {
			return model.ChannelSub2APIState{}, errors.New("sub2api password is required when username is set")
		}
		return state, nil
	}
	if origin.Sub2APIUsername != username {
		if password == "" {
			return model.ChannelSub2APIState{}, errors.New("sub2api password is required when username changes")
		}
		return state, nil
	}
	if password != "" {
		return state, nil
	}
	return origin.Sub2APIState(), nil
}
