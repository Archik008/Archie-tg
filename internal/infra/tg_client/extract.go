package tgclient

import (
	"fmt"

	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/gotd/td/tg"
)

// AccountFromMessage extracts sender account from any incoming private-chat message type.
func AccountFromMessage(msg tg.MessageClass, entities tg.Entities, selfUserID int64) (dto.AccountDTO, string, bool) {
	switch m := msg.(type) {
	case *tg.Message:
		return accountFromPeer(m.Out, m.PeerID, m.FromID, entities, selfUserID, messageContentKind(m))
	case *tg.MessageService:
		return accountFromPeer(m.Out, m.PeerID, m.FromID, entities, selfUserID, serviceActionKind(m.Action))
	case *tg.MessageEmpty:
		return dto.AccountDTO{}, "empty", false
	default:
		return dto.AccountDTO{}, fmt.Sprintf("%T", msg), false
	}
}

func accountFromPeer(
	out bool,
	peerID tg.PeerClass,
	fromID tg.PeerClass,
	entities tg.Entities,
	selfUserID int64,
	kind string,
) (dto.AccountDTO, string, bool) {
	if out {
		return dto.AccountDTO{}, kind, false
	}

	userID, ok := privatePeerUserID(peerID)
	if !ok {
		return dto.AccountDTO{}, kind, false
	}
	if int64(userID) == selfUserID {
		return dto.AccountDTO{}, kind, false
	}

	username := ""
	accessHash := int64(0)
	if user, ok := entities.Users[int64(userID)]; ok {
		username = user.Username
		accessHash = user.AccessHash
	}

	// Some service messages may carry sender in FromID.
	if fromID != nil {
		if fromUserID, ok := privatePeerUserID(fromID); ok && fromUserID != userID {
			if user, ok := entities.Users[int64(fromUserID)]; ok {
				userID = fromUserID
				username = user.Username
				accessHash = user.AccessHash
			}
		}
	}

	return dto.AccountDTO{
		UserID:     userID,
		Username:   username,
		AccessHash: accessHash,
	}, kind, true
}

func privatePeerUserID(peer tg.PeerClass) (int, bool) {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return int(p.UserID), true
	default:
		return 0, false
	}
}

func messageContentKind(m *tg.Message) string {
	if m.Media == nil {
		if m.Message != "" {
			return "text"
		}
		return "message"
	}

	switch m.Media.(type) {
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaDocument:
		return "document"
	case *tg.MessageMediaGeo:
		return "geo"
	case *tg.MessageMediaContact:
		return "contact"
	case *tg.MessageMediaVenue:
		return "venue"
	case *tg.MessageMediaGeoLive:
		return "geo_live"
	case *tg.MessageMediaPoll:
		return "poll"
	case *tg.MessageMediaDice:
		return "dice"
	case *tg.MessageMediaGame:
		return "game"
	case *tg.MessageMediaInvoice:
		return "invoice"
	case *tg.MessageMediaStory:
		return "story"
	case *tg.MessageMediaWebPage:
		return "webpage"
	case *tg.MessageMediaUnsupported:
		return "unsupported"
	case *tg.MessageMediaGiveaway:
		return "giveaway"
	case *tg.MessageMediaGiveawayResults:
		return "giveaway_results"
	case *tg.MessageMediaPaidMedia:
		return "paid_media"
	default:
		return fmt.Sprintf("media:%T", m.Media)
	}
}

func serviceActionKind(action tg.MessageActionClass) string {
	if action == nil {
		return "service"
	}
	return fmt.Sprintf("service:%T", action)
}
