package service

import (
	"path/filepath"
	"testing"

	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities/clan"
	"github.com/talesmud/talesmud/pkg/repository"
)

func newClanTestFacade(t *testing.T) Facade {
	t.Helper()
	client, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return NewFacade(repository.NewSQLiteFactory(client), nil)
}

func TestClanService_CreateClanAndMembership(t *testing.T) {
	facade := newClanTestFacade(t)
	cs := facade.ClansService()

	cl, err := cs.CreateClan("char-1", "Dark Sun", "DRK", "description")
	if err != nil {
		t.Fatalf("create clan: %v", err)
	}
	if cl.ID == "" || cl.Name != "Dark Sun" {
		t.Fatalf("unexpected clan: %+v", cl)
	}

	// One character cannot be in two clans
	_, err = cs.CreateClan("char-1", "Other", "OTH", "")
	if err == nil {
		t.Fatal("expected error when creating clan while already in clan")
	}

	// char-1 should be in clan
	inClan, err := cs.IsCharacterInClan("char-1")
	if err != nil || !inClan {
		t.Fatalf("IsCharacterInClan: %v %v", err, inClan)
	}

	// Get clan for character
	c2, err := cs.GetClanForCharacter("char-1")
	if err != nil || c2.ID != cl.ID {
		t.Fatalf("GetClanForCharacter: %v %v", err, c2)
	}

	members, err := cs.ListMembers(cl.ID)
	if err != nil || len(members) != 1 {
		t.Fatalf("members: %v len=%d", err, len(members))
	}
}
func TestClanService_InviteAcceptKickDisband(t *testing.T) {
	facade := newClanTestFacade(t)
	cs := facade.ClansService()

	cl, err := cs.CreateClan("char-leader", "Alpha", "ALP", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	inv, err := cs.Invite(cl.ID, "char-leader", "char-member")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}

	// Accept invite
	if err := cs.AcceptInvite(inv.ID, "char-member"); err != nil {
		t.Fatalf("accept: %v", err)
	}

	members, err := cs.ListMembers(cl.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("members after accept: %v len=%d", err, len(members))
	}

	// Kick - leader cannot kick self
	if err := cs.Kick(cl.ID, "char-leader", "char-leader"); err == nil {
		t.Fatal("expected error kicking self")
	}
}
func TestClanService_WithdrawOnlyLeader(t *testing.T) {
	facade := newClanTestFacade(t)
	cs := facade.ClansService()

	cl, err := cs.CreateClan("char-leader", "Beta", "BET", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	inv, err := cs.Invite(cl.ID, "char-leader", "char-m")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if err := cs.AcceptInvite(inv.ID, "char-m"); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Deposit
	if err := cs.Deposit(cl.ID, "char-leader", 100); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	bal, _ := cs.GetBankBalance(cl.ID)
	if bal != 100 {
		t.Fatalf("bal: %d", bal)
	}

	// Member cannot withdraw
	if err := cs.Withdraw(cl.ID, "char-m", 10); err == nil {
		t.Fatal("expected member cannot withdraw")
	}
	// Leader can withdraw
	if err := cs.Withdraw(cl.ID, "char-leader", 10); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	bal, _ = cs.GetBankBalance(cl.ID)
	if bal != 90 {
		t.Fatalf("bal after: %d", bal)
	}

	ledger, err := cs.GetBankLedger(cl.ID, 10)
	if err != nil || len(ledger) != 2 { // deposit + withdraw
		t.Fatalf("ledger: %v len=%d", err, len(ledger))
	}
}
func TestClanService_RankAndDeclineInvite(t *testing.T) {
	facade := newClanTestFacade(t)
	cs := facade.ClansService()

	cl, err := cs.CreateClan("char-leader", "Delta", "DLT", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	inv, err := cs.Invite(cl.ID, "char-leader", "char-m")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if err := cs.AcceptInvite(inv.ID, "char-m"); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Leader promotes the member to officer.
	if err := cs.ChangeRank(cl.ID, "char-leader", "char-m", clan.RankOfficer); err != nil {
		t.Fatalf("change rank: %v", err)
	}
	members, err := cs.ListMembers(cl.ID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	var promoted *clan.ClanMember
	for _, m := range members {
		if m != nil && m.CharacterID == "char-m" {
			promoted = m
		}
	}
	if promoted == nil || promoted.Rank != clan.RankOfficer {
		t.Fatalf("expected officer rank, got %+v", promoted)
	}

	// A non-leader cannot change ranks.
	if err := cs.ChangeRank(cl.ID, "char-m", "char-m", clan.RankLeader); err == nil {
		t.Fatal("expected non-leader rank change to fail")
	}

	// Decline a pending invite without joining.
	inv2, err := cs.Invite(cl.ID, "char-leader", "char-other")
	if err != nil {
		t.Fatalf("invite2: %v", err)
	}
	if err := cs.DeclineInvite(inv2.ID, "char-other"); err != nil {
		t.Fatalf("decline: %v", err)
	}
	inClan, _ := cs.IsCharacterInClan("char-other")
	if inClan {
		t.Fatal("declined character must not join the clan")
	}
}

func TestClanService_KickLeaderProhibited(t *testing.T) {
	facade := newClanTestFacade(t)
	cs := facade.ClansService()

	cl, err := cs.CreateClan("char-leader", "Gamma", "GAM", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	inv, err := cs.Invite(cl.ID, "char-leader", "char-m")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if err := cs.AcceptInvite(inv.ID, "char-m"); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Cannot kick leader
	if err := cs.Kick(cl.ID, "char-m", "char-leader"); err == nil {
		t.Fatal("expected cannot kick leader")
	}
	if err := cs.Kick(cl.ID, "char-leader", "char-m"); err != nil {
		t.Fatalf("kick member ok: %v", err)
	}
}
