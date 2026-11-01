package repository

import (
	"testing"
	"time"

	e "github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/dm"
)

func TestDMFindOrCreateChannel(t *testing.T) {
	repo := newSocialTestFactory(t).DMChannels()

	first, err := repo.FindOrCreateChannel("char-a", "char-b")
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if first.ID == "" {
		t.Fatal("expected generated channel ID")
	}
	if first.ParticipantA != "char-a" || first.ParticipantB != "char-b" {
		t.Fatalf("unexpected participants: %+v", first)
	}

	// Reversed order must resolve to the same channel (on-demand uniqueness).
	second, err := repo.FindOrCreateChannel("char-b", "char-a")
	if err != nil {
		t.Fatalf("find-or-create reversed: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected same channel, got %s and %s", first.ID, second.ID)
	}

	between, err := repo.FindChannelBetween("char-b", "char-a")
	if err != nil || between.ID != first.ID {
		t.Fatalf("find between: %v channel=%+v", err, between)
	}
	between, err = repo.FindChannelBetween("char-a", "char-b")
	if err != nil || between.ID != first.ID {
		t.Fatalf("find between canonical: %v channel=%+v", err, between)
	}

	channels, err := repo.FindChannelsByCharacter("char-a")
	if err != nil || len(channels) != 1 {
		t.Fatalf("channels by character: %v len=%d", err, len(channels))
	}

	if _, err := repo.FindOrCreateChannel("char-a", "char-a"); err == nil {
		t.Fatal("expected self-channel to fail")
	}
	if _, err := repo.FindOrCreateChannel("", "char-b"); err == nil {
		t.Fatal("expected empty participant to fail")
	}
}

func TestDMMessagesAndReadState(t *testing.T) {
	repo := newSocialTestFactory(t).DMChannels()

	channel, err := repo.FindOrCreateChannel("char-a", "char-b")
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	base := time.Now().Add(-time.Hour).Truncate(time.Millisecond)
	m1 := &dm.DMMessage{
		Entity:     &e.Entity{ID: "msg-1"},
		ChannelID:  channel.ID,
		FromCharID: "char-a",
		Body:       "hello",
		SentAt:     base,
	}
	m2 := &dm.DMMessage{
		Entity:     &e.Entity{ID: "msg-2"},
		ChannelID:  channel.ID,
		FromCharID: "char-a",
		Body:       "are you there?",
		SentAt:     base.Add(1 * time.Second),
	}
	if _, err := repo.StoreMessage(m1); err != nil {
		t.Fatalf("store m1: %v", err)
	}
	if _, err := repo.StoreMessage(m2); err != nil {
		t.Fatalf("store m2: %v", err)
	}

	if err := repo.UpdateLastMessage(channel.ID, m2.ID, m2.SentAt); err != nil {
		t.Fatalf("update last message: %v", err)
	}
	if err := repo.IncrementUnread(channel.ID, "char-b"); err != nil {
		t.Fatalf("increment unread: %v", err)
	}
	if err := repo.IncrementUnread(channel.ID, "char-b"); err != nil {
		t.Fatalf("increment unread: %v", err)
	}
	unread, err := repo.GetUnreadCount(channel.ID, "char-b")
	if err != nil || unread != 2 {
		t.Fatalf("unread count: %v count=%d", err, unread)
	}

	all, err := repo.FindMessagesByChannel(channel.ID, 10)
	if err != nil || len(all) != 2 {
		t.Fatalf("messages by channel: %v len=%d", err, len(all))
	}
	// Oldest first.
	if all[0].ID != m1.ID || all[1].ID != m2.ID {
		t.Fatalf("unexpected message order: %s, %s", all[0].ID, all[1].ID)
	}

	since, err := repo.FindMessagesSince(channel.ID, base.Add(500*time.Millisecond))
	if err != nil || len(since) != 1 || since[0].ID != m2.ID {
		t.Fatalf("messages since: %v len=%d", err, len(since))
	}
	// Nothing newer than the last message.
	since, err = repo.FindMessagesSince(channel.ID, m2.SentAt)
	if err != nil || len(since) != 0 {
		t.Fatalf("expected no messages since m2: %v len=%d", err, len(since))
	}

	// Per-message read.
	if err := repo.MarkMessageRead(m1.ID); err != nil {
		t.Fatalf("mark message read: %v", err)
	}
	read, err := repo.FindMessageByID(m1.ID)
	if err != nil || !read.IsRead() {
		t.Fatalf("expected m1 read: %v msg=%+v", err, read)
	}

	// Channel-level read clears the counter and marks the other side's messages.
	if err := repo.MarkRead(channel.ID, "char-b"); err != nil {
		t.Fatalf("mark channel read: %v", err)
	}
	unread, _ = repo.GetUnreadCount(channel.ID, "char-b")
	if unread != 0 {
		t.Fatalf("expected unread to reset, got %d", unread)
	}
	m2Read, _ := repo.FindMessageByID(m2.ID)
	if !m2Read.IsRead() {
		t.Fatal("expected m2 to be marked read after MarkRead")
	}

	if err := repo.DeleteMessage(m2.ID); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	if _, err := repo.FindMessageByID(m2.ID); err == nil {
		t.Fatal("expected deleted message to be gone")
	}
}

var _ DMRepository = (*sqliteDMChannelRepository)(nil)
