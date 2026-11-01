package service

import (
	"errors"
	"time"

	"github.com/talesmud/talesmud/pkg/entities/dm"
	r "github.com/talesmud/talesmud/pkg/repository"
)

// DMService provides business logic for managing direct messages
type DMService interface {
	GetChannels(characterID string) ([]*dm.DMChannel, error)
	FindOrCreateChannel(a, b string) (*dm.DMChannel, error)
	FindChannelBetween(a, b string) (*dm.DMChannel, error)
	SendMessage(channelID, fromCharID, body string) (*dm.DMMessage, error)
	GetMessages(channelID, characterID string, limit int, since *time.Time) ([]*dm.DMMessage, error)
	MarkRead(channelID, characterID string) error
}

type dmService struct {
	repo r.DMRepository
}

// NewDMService creates a new DM service.
func NewDMService(dmRepo r.DMRepository) DMService {
	return &dmService{
		repo: dmRepo,
	}
}

func (s *dmService) GetChannels(characterID string) ([]*dm.DMChannel, error) {
	if characterID == "" {
		return nil, errors.New("character id is required")
	}
	return s.repo.FindChannelsByCharacter(characterID)
}

func (s *dmService) FindOrCreateChannel(a, b string) (*dm.DMChannel, error) {
	if a == "" || b == "" {
		return nil, errors.New("participants are required")
	}
	if a == b {
		return nil, errors.New("cannot dm self")
	}
	return s.repo.FindOrCreateChannel(a, b)
}

func (s *dmService) FindChannelBetween(a, b string) (*dm.DMChannel, error) {
	if a == "" || b == "" {
		return nil, errors.New("participants are required")
	}
	return s.repo.FindChannelBetween(a, b)
}

func (s *dmService) SendMessage(channelID, fromCharID, body string) (*dm.DMMessage, error) {
	if channelID == "" || fromCharID == "" {
		return nil, errors.New("invalid message parameters")
	}
	if body == "" {
		return nil, errors.New("message body is required")
	}

	ch, err := s.repo.FindByID(channelID)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, errors.New("channel not found")
	}

	// Validate that fromCharID is participant (anti-spoof)
	if ch.ParticipantA != fromCharID && ch.ParticipantB != fromCharID {
		return nil, errors.New("not a participant of this channel")
	}

	msg := &dm.DMMessage{
		ChannelID:  channelID,
		FromCharID: fromCharID,
		Body:       body,
		SentAt:     time.Now(),
	}
	stored, err := s.repo.StoreMessage(msg)
	if err != nil {
		return nil, err
	}

	// Determine recipient and increment unread
	recipient := ch.ParticipantA
	if ch.ParticipantA == fromCharID {
		recipient = ch.ParticipantB
	}

	if err := s.repo.IncrementUnread(channelID, recipient); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateLastMessage(channelID, stored.ID, stored.SentAt); err != nil {
		return nil, err
	}

	return stored, nil
}

func (s *dmService) GetMessages(channelID, characterID string, limit int, since *time.Time) ([]*dm.DMMessage, error) {
	if channelID == "" || characterID == "" {
		return nil, errors.New("invalid parameters")
	}

	ch, err := s.repo.FindByID(channelID)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, errors.New("channel not found")
	}
	if ch.ParticipantA != characterID && ch.ParticipantB != characterID {
		return nil, errors.New("not a participant of this channel")
	}

	if since != nil && !since.IsZero() {
		return s.repo.FindMessagesSince(channelID, *since)
	}
	return s.repo.FindMessagesByChannel(channelID, limit)
}

func (s *dmService) MarkRead(channelID, characterID string) error {
	if channelID == "" || characterID == "" {
		return errors.New("invalid parameters")
	}

	ch, err := s.repo.FindByID(channelID)
	if err != nil {
		return err
	}
	if ch == nil {
		return errors.New("channel not found")
	}
	if ch.ParticipantA != characterID && ch.ParticipantB != characterID {
		return errors.New("not a participant of this channel")
	}

	return s.repo.MarkRead(channelID, characterID)
}
