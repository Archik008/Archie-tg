package tgclient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/tg"
)

// BeginAuthResult contains the exact credentials and delivery method used for SendCode.
type BeginAuthResult struct {
	Phone         string
	AppID         int
	AppHashPrefix string
	CodeDelivery  string
	CodeHint      string
	ResendAfter   int
}

type authAuditRecord struct {
	Time          time.Time `json:"time"`
	AppID         int       `json:"app_id"`
	AppHashPrefix string    `json:"app_hash_prefix"`
	Phone         string    `json:"phone"`
	CodeDelivery  string    `json:"code_delivery,omitempty"`
	Error         string    `json:"error,omitempty"`
}

func (c *Client) writeAuthAudit(record authAuditRecord) {
	record.Time = time.Now().UTC()
	path := filepath.Join(c.sessionDir, "auth.last.json")
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func maskAppHash(appHash string) string {
	appHash = strings.TrimSpace(appHash)
	if len(appHash) <= 8 {
		return appHash
	}
	return appHash[:8] + "..."
}

func validateAppCredentials(appID int, appHash string) error {
	appHash = strings.TrimSpace(appHash)
	if appID <= 0 {
		return fmt.Errorf("app_id must be a positive number")
	}
	if len(appHash) != 32 {
		return fmt.Errorf("app_hash must be exactly 32 characters")
	}
	for _, ch := range appHash {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return fmt.Errorf("app_hash must contain only hex characters")
		}
	}
	return nil
}

func codeDeliveryMessage(sent tg.AuthSentCodeClass) string {
	switch s := sent.(type) {
	case *tg.AuthSentCode:
		switch t := s.Type.(type) {
		case *tg.AuthSentCodeTypeApp:
			if t.Length > 0 {
				return fmt.Sprintf("Telegram app (%d digits)", t.Length)
			}
			return "Telegram app"
		case *tg.AuthSentCodeTypeSMS:
			return "SMS"
		case *tg.AuthSentCodeTypeCall:
			return "phone call"
		case *tg.AuthSentCodeTypeMissedCall:
			if t.Prefix != "" {
				return fmt.Sprintf("missed call, last digits %s", t.Prefix)
			}
			return "missed call"
		case *tg.AuthSentCodeTypeFragmentSMS:
			return "Fragment SMS"
		default:
			return fmt.Sprintf("%T", t)
		}
	case *tg.AuthSentCodeSuccess:
		return "already authorized"
	default:
		return fmt.Sprintf("%T", sent)
	}
}

func codeDeliveryHint(sent tg.AuthSentCodeClass) string {
	switch s := sent.(type) {
	case *tg.AuthSentCode:
		switch s.Type.(type) {
		case *tg.AuthSentCodeTypeApp:
			return "Check Telegram app on a device with this account. Code is in chat from Telegram, not SMS."
		case *tg.AuthSentCodeTypeSMS:
			return "Check SMS on this phone."
		case *tg.AuthSentCodeTypeCall, *tg.AuthSentCodeTypeMissedCall:
			return "Wait for a call; the code is spoken or shown as missed-call digits."
		default:
			return "Wait for the code using the delivery method shown above."
		}
	default:
		return ""
	}
}

func nextDeliveryMessage(next tg.AuthCodeTypeClass) string {
	if next == nil {
		return ""
	}
	switch next.(type) {
	case *tg.AuthCodeTypeSMS:
		return "SMS"
	case *tg.AuthCodeTypeCall:
		return "phone call"
	case *tg.AuthCodeTypeFlashCall:
		return "flash call"
	default:
		return fmt.Sprintf("%T", next)
	}
}
