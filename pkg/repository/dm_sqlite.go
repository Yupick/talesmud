package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbsqlite "github.com/talesmud/talesmud/pkg/db/sqlite"
	e "github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/dm"
)

type sqliteDMChannelRepository struct {
	db *sql.DB
}

// NewSQLiteDMChannelRepository creates a new SQLite DM channel repository.
//
// dm_messages is the one table that deviates from the pure (id, data) JSON
// pattern: it carries real channel_id / sent_at / from_char_id columns so range
// and ordering queries can use indexes. Those columns are always written in the
// same statement as the JSON payload and mirror its values.
func NewSQLiteDMChannelRepository(client *dbsqlite.Client) DMRepository {
	return &sqliteDMChannelRepository{db: client.DB()}
}

// canonicalParticipants orders two character IDs so a pair always maps to a
// single channel regardless of who started the conversation.
func canonicalParticipants(a, b string) (string, string) {
	if a <= b {
		return a, b
	}
	return b, a
}

// ---------- Channels ----------

func (repo *sqliteDMChannelRepository) Drop() error {
	for _, table := range []string{"dm_channels", "dm_messages"} {
		if _, err := repo.db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			return err
		}
	}
	return nil
}

func (repo *sqliteDMChannelRepository) channelByID(id string) (*dm.DMChannel, error) {
	row := repo.db.QueryRow(`SELECT data FROM dm_channels WHERE id = ?`, id)
	var payload string
	if err := row.Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("dm channel not found")
		}
		return nil, err
	}
	channel := &dm.DMChannel{}
	if err := json.Unmarshal([]byte(payload), channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (repo *sqliteDMChannelRepository) FindByID(id string) (*dm.DMChannel, error) {
	return repo.channelByID(id)
}

// lookupChannel returns the channel between a and b, or (nil, nil) if none.
func (repo *sqliteDMChannelRepository) lookupChannel(a, b string) (*dm.DMChannel, error) {
	pa, pb := canonicalParticipants(a, b)
	row := repo.db.QueryRow(
		`SELECT data FROM dm_channels
		 WHERE json_extract(data, '$.participantA') = ?
		   AND json_extract(data, '$.participantB') = ?
		 LIMIT 1`,
		pa,
		pb,
	)
	var payload string
	if err := row.Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	channel := &dm.DMChannel{}
	if err := json.Unmarshal([]byte(payload), channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (repo *sqliteDMChannelRepository) FindChannelBetween(a, b string) (*dm.DMChannel, error) {
	channel, err := repo.lookupChannel(a, b)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, errors.New("dm channel not found")
	}
	return channel, nil
}

// FindOrCreateChannel creates the channel on demand (first message between two
// characters) or returns the existing one.
func (repo *sqliteDMChannelRepository) FindOrCreateChannel(a, b string) (*dm.DMChannel, error) {
	if a == "" || b == "" {
		return nil, errors.New("participants are required")
	}
	if a == b {
		return nil, errors.New("cannot create a dm channel with the same character")
	}
	channel, err := repo.lookupChannel(a, b)
	if err != nil {
		return nil, err
	}
	if channel != nil {
		return channel, nil
	}
	pa, pb := canonicalParticipants(a, b)
	channel = &dm.DMChannel{
		Entity:       e.NewEntity(),
		ParticipantA: pa,
		ParticipantB: pb,
		CreatedAt:    time.Now(),
	}
	if err := repo.storeChannel(channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (repo *sqliteDMChannelRepository) storeChannel(channel *dm.DMChannel) error {
	if channel == nil {
		return errors.New("channel is nil")
	}
	payload, err := json.Marshal(channel)
	if err != nil {
		return err
	}
	_, err = repo.db.Exec(
		`INSERT INTO dm_channels (id, data) VALUES (?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data`,
		channel.ID,
		string(payload),
	)
	return err
}

func (repo *sqliteDMChannelRepository) FindChannelsByCharacter(characterID string) ([]*dm.DMChannel, error) {
	rows, err := repo.db.Query(
		`SELECT data FROM dm_channels
		 WHERE json_extract(data, '$.participantA') = ?
		    OR json_extract(data, '$.participantB') = ?
		 ORDER BY json_extract(data, '$.lastMessageAt') DESC`,
		characterID,
		characterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*dm.DMChannel, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		channel := &dm.DMChannel{}
		if err := json.Unmarshal([]byte(payload), channel); err != nil {
			continue
		}
		out = append(out, channel)
	}
	return out, rows.Err()
}

func (repo *sqliteDMChannelRepository) UpdateLastMessage(channelID, messageID string, timestamp time.Time) error {
	channel, err := repo.channelByID(channelID)
	if err != nil {
		return err
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	channel.LastMessageAt = timestamp
	channel.LastMessageID = messageID
	return repo.storeChannel(channel)
}

func (repo *sqliteDMChannelRepository) IncrementUnread(channelID, recipient string) error {
	channel, err := repo.channelByID(channelID)
	if err != nil {
		return err
	}
	switch recipient {
	case channel.ParticipantA:
		channel.UnreadA++
	case channel.ParticipantB:
		channel.UnreadB++
	default:
		return fmt.Errorf("character %s is not a participant of channel %s", recipient, channelID)
	}
	return repo.storeChannel(channel)
}

// MarkRead zeroes the reader's unread counter and stamps every message they did
// not send as read.
func (repo *sqliteDMChannelRepository) MarkRead(channelID, reader string) error {
	channel, err := repo.channelByID(channelID)
	if err != nil {
		return err
	}
	switch reader {
	case channel.ParticipantA:
		channel.UnreadA = 0
	case channel.ParticipantB:
		channel.UnreadB = 0
	default:
		return fmt.Errorf("character %s is not a participant of channel %s", reader, channelID)
	}
	if err := repo.storeChannel(channel); err != nil {
		return err
	}
	return repo.markMessagesRead(channelID, reader)
}

// markMessagesRead sets readAt on all messages in a channel not sent by reader.
func (repo *sqliteDMChannelRepository) markMessagesRead(channelID, reader string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := repo.db.Exec(
		`UPDATE dm_messages
		 SET data = json_set(data, '$.readAt', ?)
		 WHERE channel_id = ?
		   AND from_char_id != ?
		   AND json_extract(data, '$.readAt') IS NULL`,
		now,
		channelID,
		reader,
	)
	return err
}

func (repo *sqliteDMChannelRepository) GetUnreadCount(channelID, reader string) (int, error) {
	channel, err := repo.channelByID(channelID)
	if err != nil {
		return 0, err
	}
	switch reader {
	case channel.ParticipantA:
		return channel.UnreadA, nil
	case channel.ParticipantB:
		return channel.UnreadB, nil
	default:
		return 0, fmt.Errorf("character %s is not a participant of channel %s", reader, channelID)
	}
}

// ---------- Messages ----------

func (repo *sqliteDMChannelRepository) StoreMessage(msg *dm.DMMessage) (*dm.DMMessage, error) {
	if msg == nil {
		return nil, errors.New("message is nil")
	}
	if msg.ChannelID == "" || msg.FromCharID == "" {
		return nil, errors.New("message requires channelID and fromCharID")
	}
	if msg.Entity == nil {
		msg.Entity = e.NewEntity()
	}
	if msg.SentAt.IsZero() {
		msg.SentAt = time.Now()
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	_, err = repo.db.Exec(
		`INSERT INTO dm_messages (id, data, channel_id, sent_at, from_char_id)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		     data = excluded.data,
		     channel_id = excluded.channel_id,
		     sent_at = excluded.sent_at,
		     from_char_id = excluded.from_char_id`,
		msg.ID,
		string(payload),
		msg.ChannelID,
		msg.SentAt.UnixMilli(),
		msg.FromCharID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store dm message: %w", err)
	}
	return msg, nil
}

func (repo *sqliteDMChannelRepository) FindMessageByID(id string) (*dm.DMMessage, error) {
	row := repo.db.QueryRow(`SELECT data FROM dm_messages WHERE id = ?`, id)
	var payload string
	if err := row.Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("dm message not found")
		}
		return nil, err
	}
	msg := &dm.DMMessage{}
	if err := json.Unmarshal([]byte(payload), msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (repo *sqliteDMChannelRepository) scanMessages(rows *sql.Rows) ([]*dm.DMMessage, error) {
	out := make([]*dm.DMMessage, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		msg := &dm.DMMessage{}
		if err := json.Unmarshal([]byte(payload), msg); err != nil {
			continue
		}
		out = append(out, msg)
	}
	return out, rows.Err()
}

// FindMessagesByChannel returns the latest `limit` messages in chronological
// (oldest first) order.
func (repo *sqliteDMChannelRepository) FindMessagesByChannel(channelID string, limit int) ([]*dm.DMMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := repo.db.Query(
		`SELECT data FROM dm_messages
		 WHERE channel_id = ?
		 ORDER BY sent_at DESC
		 LIMIT ?`,
		channelID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	descending, err := repo.scanMessages(rows)
	if err != nil {
		return nil, err
	}
	// Reverse so callers get oldest-first order.
	for i, j := 0, len(descending)-1; i < j; i, j = i+1, j-1 {
		descending[i], descending[j] = descending[j], descending[i]
	}
	return descending, nil
}

// FindMessagesSince returns messages sent strictly after `since`, oldest first.
// It uses the real channel_id / sent_at columns (and their index), not
// json_extract.
func (repo *sqliteDMChannelRepository) FindMessagesSince(channelID string, since time.Time) ([]*dm.DMMessage, error) {
	rows, err := repo.db.Query(
		`SELECT data FROM dm_messages
		 WHERE channel_id = ? AND sent_at > ?
		 ORDER BY sent_at ASC`,
		channelID,
		since.UnixMilli(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return repo.scanMessages(rows)
}

func (repo *sqliteDMChannelRepository) MarkMessageRead(messageID string) error {
	msg, err := repo.FindMessageByID(messageID)
	if err != nil {
		return err
	}
	msg.MarkRead()
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	result, err := repo.db.Exec(`UPDATE dm_messages SET data = ? WHERE id = ?`, string(payload), messageID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("dm message not found")
	}
	return nil
}

func (repo *sqliteDMChannelRepository) DeleteMessage(messageID string) error {
	result, err := repo.db.Exec(`DELETE FROM dm_messages WHERE id = ?`, messageID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("dm message not found")
	}
	return nil
}
