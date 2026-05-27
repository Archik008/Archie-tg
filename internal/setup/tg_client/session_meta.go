package tgclient

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type SessionMeta struct {
	AppID      int    `json:"app_id"`
	AppHash    string `json:"app_hash"`
	Phone      string `json:"phone"`
	SelfUserID int64  `json:"self_user_id,omitempty"`
}

func (c *Client) metaPath() string {
	return filepath.Join(filepath.Dir(c.sessionPath), "session.meta.json")
}

func (c *Client) SaveSessionMeta(meta SessionMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.metaPath(), data, 0o600)
}

func (c *Client) LoadSessionMeta() (SessionMeta, error) {
	data, err := os.ReadFile(c.metaPath())
	if err != nil {
		return SessionMeta{}, err
	}
	var meta SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return SessionMeta{}, err
	}
	if meta.AppID <= 0 || meta.AppHash == "" {
		return SessionMeta{}, errors.New("session meta is incomplete")
	}
	return meta, nil
}
