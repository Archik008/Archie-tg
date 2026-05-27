package tguserbot

import (
	"errors"

	"github.com/archik008/archie-tg/config"
	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/gotd/td/tg"
)

var ErrMissingAccessHash = errors.New("missing user access hash")

func inputPeerUser(user dto.AccountDTO, cfg config.TelegramUserCfg) (*tg.InputPeerUser, error) {
	accessHash := user.AccessHash
	if accessHash == 0 {
		// Keep backward compatibility for legacy single-user config.
		// If USER_ACCESS_HASH is also empty, caller must stop and surface error.
		accessHash = cfg.USER_ACCESS_HASH
	}
	if accessHash == 0 {
		return nil, ErrMissingAccessHash
	}
	return &tg.InputPeerUser{
		UserID:     int64(user.UserID),
		AccessHash: accessHash,
	}, nil
}
