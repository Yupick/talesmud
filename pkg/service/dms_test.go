package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/repository"
)

func newDMTestFacade(t *testing.T) Facade {
	t.Helper()
	client, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return NewFacade(repository.NewSQLiteFactory(client), nil)
}

func TestDMService_UniqueChannelPerPair_SendAntiSpoof(t *testing.T) {
	facade := newDMTestFacade(t)
	ds := facade.DMService()

	ch1, err := ds.FindOrCreateChannel("a", "b")
	if err != nil {
		t.Fatalf("findorcreate: %v", err)
	}
	ch2, err := ds.FindOrCreateChannel("b", "a")
	if err != nil {
		t.Fatalf("findorcreate2: %v", err)
	}
	if ch1.ID != ch2.ID {
		t.Fatal("expected same channel for pair")
	}

	// Send message as b (participant) - ok
	msg, err := ds.SendMessage(ch1.ID, "b", "hi")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if msg.Body != "hi" || msg.FromCharID != "b" {
		t.Fatalf("bad msg: %+v", msg)
	}

	// Anti-spoof: send as non-participant
	_, err = ds.SendMessage(ch1.ID, "c", "hack")
	if err == nil {
		t.Fatal("expected anti-spoof error")
	}

	// Get messages as participant a
	msgs, err := ds.GetMessages(ch1.ID, "a", 10, nil)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("get msgs: %v len=%d", err, len(msgs))
	}

	// Get messages as non-participant c - should fail
	_, err = ds.GetMessages(ch1.ID, "c", 10, nil)
	if err == nil {
		t.Fatal("expected get messages fails for non-participant")
	}
}
func TestDMService_MarkRead(t *testing.T) {
	facade := newDMTestFacade(t)
	ds := facade.DMService()

	ch, err := ds.FindOrCreateChannel("a", "b")
	if err != nil {
		t.Fatalf("chan: %v", err)
	}

	if _, err := ds.SendMessage(ch.ID, "a", "m1"); err != nil {
		t.Fatalf("send1: %v", err)
	}
	if _, err := ds.SendMessage(ch.ID, "a", "m2"); err != nil {
		t.Fatalf("send2: %v", err)
	}

	channels, err := ds.GetChannels("b")
	if err != nil || len(channels) != 1 {
		t.Fatalf("channels b: %v len=%d", err, len(channels))
	}
	if channels[0].UnreadB != 2 {
		t.Fatalf("unreadB expected 2, got %d", channels[0].UnreadB)
	}

	if err := ds.MarkRead(ch.ID, "b"); err != nil {
		t.Fatalf("markread: %v", err)
	}
	channels, _ = ds.GetChannels("b")
	if channels[0].UnreadB != 0 {
		t.Fatalf("unreadB after markread: %d", channels[0].UnreadB)
	}

	// Non-participant cannot mark read
	if err := ds.MarkRead(ch.ID, "c"); err == nil {
		t.Fatal("expected markread fails for non-participant")
	}
}

func TestDMService_GetMessagesSince(t *testing.T) {
	facade := newDMTestFacade(t)
	ds := facade.DMService()

	ch, err := ds.FindOrCreateChannel("a", "b")
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	if _, err := ds.SendMessage(ch.ID, "a", "old"); err != nil {
		t.Fatalf("send: %v", err)
	}

	// A future `since` timestamp excludes the earlier message.
	future := time.Now().Add(time.Hour)
	msgs, err := ds.GetMessages(ch.ID, "b", 10, &future)
	if err != nil {
		t.Fatalf("get messages since: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no messages after future since, got %d", len(msgs))
	}
}
