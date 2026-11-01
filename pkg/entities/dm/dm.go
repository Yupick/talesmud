// Package dm holds the entities backing the Phase 4 private messaging system:
// a conversation channel between two characters and the messages inside it.
package dm

import (
	"time"

	"github.com/talesmud/talesmud/pkg/entities"
)

// DMChannel is a private conversation between two characters.
//
// Participants are stored in canonical (lexicographically sorted) order so a
// pair of characters always maps to at most one channel.
type DMChannel struct {
	*entities.Entity `json:",inline"`

	ParticipantA  string    `json:"participantA"` // characterID
	ParticipantB  string    `json:"participantB"` // characterID
	CreatedAt     time.Time `json:"createdAt"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	LastMessageID string    `json:"lastMessageID"`
	UnreadA       int       `json:"unreadA"` // Messages unread by A
	UnreadB       int       `json:"unreadB"` // Messages unread by B
}

// DMMessage is a single message inside a DM channel.
type DMMessage struct {
	*entities.Entity `json:",inline"`

	ChannelID  string     `json:"channelID"`
	FromCharID string     `json:"fromCharID"`
	Body       string     `json:"body"`
	SentAt     time.Time  `json:"sentAt"`
	ReadAt     *time.Time `json:"readAt,omitempty"` // nil = unread
}

// GetReadAt returns the read timestamp or nil when unread.
func (m *DMMessage) GetReadAt() *time.Time {
	return m.ReadAt
}

// IsRead reports whether the message has been read.
func (m *DMMessage) IsRead() bool {
	return m.ReadAt != nil
}

// MarkRead stamps the message as read at the current time.
func (m *DMMessage) MarkRead() {
	now := time.Now()
	m.ReadAt = &now
}

// Doc-compatible aliases. The Phase 4 schema design refers to these entities as
// dm.Channel and dm.Message.
type (
	// Channel is an alias for DMChannel.
	Channel = DMChannel
	// Message is an alias for DMMessage.
	Message = DMMessage
)
