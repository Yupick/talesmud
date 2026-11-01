package repository

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities/clan"
)

// newSocialTestFactory opens an in-memory SQLite database and returns a factory
// wired to it. The connection lives for the duration of the test.
func newSocialTestFactory(t *testing.T) Factory {
	t.Helper()
	client, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return NewSQLiteFactory(client)
}

func TestClanStoreAndFinders(t *testing.T) {
	repo := newSocialTestFactory(t).Clans()

	stored, err := repo.Store(&clan.Clan{
		Name:         "Dark Sun",
		Tag:          "DRK",
		LeaderCharID: "char-leader",
		Active:       true,
		MaxMembers:   20,
	})
	if err != nil {
		t.Fatalf("store clan: %v", err)
	}
	if stored.Entity == nil || stored.ID == "" {
		t.Fatal("expected generated clan ID")
	}
	if stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be set")
	}

	byID, err := repo.FindByID(stored.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if byID.Name != "Dark Sun" || byID.Tag != "DRK" {
		t.Fatalf("unexpected clan: %+v", byID)
	}

	byName, err := repo.FindByName("Dark Sun")
	if err != nil || len(byName) != 1 {
		t.Fatalf("find by name: %v len=%d", err, len(byName))
	}
	byTag, err := repo.FindByTag("DRK")
	if err != nil || len(byTag) != 1 {
		t.Fatalf("find by tag: %v len=%d", err, len(byTag))
	}
	byLeader, err := repo.FindByLeader("char-leader")
	if err != nil || len(byLeader) != 1 {
		t.Fatalf("find by leader: %v len=%d", err, len(byLeader))
	}

	all, err := repo.FindAll()
	if err != nil || len(all) != 1 {
		t.Fatalf("find all: %v len=%d", err, len(all))
	}
	active, err := repo.FindAllActive()
	if err != nil || len(active) != 1 {
		t.Fatalf("find all active: %v len=%d", err, len(active))
	}

	byID.Active = false
	if err := repo.Update(byID.ID, byID); err != nil {
		t.Fatalf("update clan: %v", err)
	}
	active, err = repo.FindAllActive()
	if err != nil || len(active) != 0 {
		t.Fatalf("expected no active clans after dissolve: %v len=%d", err, len(active))
	}
}

func TestClanMembershipUniqueness(t *testing.T) {
	repo := newSocialTestFactory(t).Clans()

	clanA, err := repo.Store(&clan.Clan{Name: "Alpha", Tag: "ALP", LeaderCharID: "leader-a", Active: true})
	if err != nil {
		t.Fatalf("store clan A: %v", err)
	}
	clanB, err := repo.Store(&clan.Clan{Name: "Beta", Tag: "BET", LeaderCharID: "leader-b", Active: true})
	if err != nil {
		t.Fatalf("store clan B: %v", err)
	}

	if err := repo.AddMember(&clan.ClanMember{CharacterID: "char-1", ClanID: clanA.ID, Rank: clan.RankLeader}); err != nil {
		t.Fatalf("add first member: %v", err)
	}
	if err := repo.AddMember(&clan.ClanMember{CharacterID: "char-2", ClanID: clanA.ID, Rank: clan.RankMember}); err != nil {
		t.Fatalf("add second member: %v", err)
	}

	// Same character can only belong to one clan.
	if err := repo.AddMember(&clan.ClanMember{CharacterID: "char-1", ClanID: clanB.ID}); err == nil {
		t.Fatal("expected adding a character to a second clan to fail")
	}
	if err := repo.AddMember(&clan.ClanMember{CharacterID: "char-1", ClanID: clanA.ID}); err == nil {
		t.Fatal("expected re-adding an existing member to fail")
	}

	member, err := repo.FindMember(clanA.ID, "char-1")
	if err != nil {
		t.Fatalf("find member: %v", err)
	}
	if member.Rank != clan.RankLeader {
		t.Fatalf("unexpected rank: %v", member.Rank)
	}

	members, err := repo.FindMembersByClan(clanA.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("members by clan: %v len=%d", err, len(members))
	}

	if err := repo.UpdateMemberRank(clanA.ID, "char-2", clan.RankOfficer); err != nil {
		t.Fatalf("update rank: %v", err)
	}
	if err := repo.UpdateMemberContribution(clanA.ID, "char-2", 250); err != nil {
		t.Fatalf("update contribution: %v", err)
	}
	member, _ = repo.FindMember(clanA.ID, "char-2")
	if member.Rank != clan.RankOfficer || member.Contribution != 250 {
		t.Fatalf("unexpected member after updates: %+v", member)
	}

	inClan, err := repo.IsCharacterInClan("char-1")
	if err != nil || !inClan {
		t.Fatalf("IsCharacterInClan: %v in=%v", err, inClan)
	}
	charClan, err := repo.GetCharacterClan("char-1")
	if err != nil || charClan.ID != clanA.ID {
		t.Fatalf("GetCharacterClan: %v clan=%+v", err, charClan)
	}

	if err := repo.RemoveMember(clanA.ID, "char-1"); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if _, err := repo.FindMember(clanA.ID, "char-1"); err == nil {
		t.Fatal("expected member to be gone after removal")
	}
	inClan, _ = repo.IsCharacterInClan("char-1")
	if inClan {
		t.Fatal("expected character to be clan-less after removal")
	}
}

func TestClanInviteLifecycle(t *testing.T) {
	repo := newSocialTestFactory(t).Clans()
	cl, err := repo.Store(&clan.Clan{Name: "Gamma", Tag: "GAM", Active: true})
	if err != nil {
		t.Fatalf("store clan: %v", err)
	}

	invite, err := repo.CreateInvite(&clan.ClanInvite{
		ClanID:      cl.ID,
		CharacterID: "char-9",
		InviterID:   "char-1",
	})
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	if invite.ID == "" || invite.Status != clan.InviteStatusPending {
		t.Fatalf("unexpected invite: %+v", invite)
	}
	if invite.ExpiresAt.IsZero() {
		t.Fatal("expected invite expiry to be set")
	}

	found, err := repo.FindInvite(cl.ID, "char-9")
	if err != nil || found.ID != invite.ID {
		t.Fatalf("find invite: %v invite=%+v", err, found)
	}
	byClan, err := repo.FindInvitesByClan(cl.ID)
	if err != nil || len(byClan) != 1 {
		t.Fatalf("invites by clan: %v len=%d", err, len(byClan))
	}
	byChar, err := repo.FindInvitesByCharacter("char-9")
	if err != nil || len(byChar) != 1 {
		t.Fatalf("invites by char: %v len=%d", err, len(byChar))
	}

	if err := repo.UpdateInviteStatus(cl.ID, "char-9", clan.InviteStatusAccepted); err != nil {
		t.Fatalf("update invite status: %v", err)
	}
	found, _ = repo.FindInvite(cl.ID, "char-9")
	if found.Status != clan.InviteStatusAccepted {
		t.Fatalf("expected accepted, got %q", found.Status)
	}

	if err := repo.DeleteInvite(cl.ID, "char-9"); err != nil {
		t.Fatalf("delete invite: %v", err)
	}
	if _, err := repo.FindInvite(cl.ID, "char-9"); err == nil {
		t.Fatal("expected invite to be gone after deletion")
	}
}

func TestClanBankLedger(t *testing.T) {
	repo := newSocialTestFactory(t).Clans()
	cl, err := repo.Store(&clan.Clan{Name: "Vault", Tag: "VLT", Active: true})
	if err != nil {
		t.Fatalf("store clan: %v", err)
	}

	if err := repo.Deposit(cl.ID, "char-1", 100, "initial"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if err := repo.Deposit(cl.ID, "char-2", 50, "gift"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	balance, err := repo.GetBankBalance(cl.ID)
	if err != nil || balance != 150 {
		t.Fatalf("balance after deposits: %v balance=%d", err, balance)
	}

	ledger, err := repo.GetBankLedger(cl.ID, 10)
	if err != nil || len(ledger) != 2 {
		t.Fatalf("ledger: %v len=%d", err, len(ledger))
	}
	// Newest first.
	if ledger[0].Type != clan.BankTypeDeposit || ledger[0].Balance != 150 {
		t.Fatalf("unexpected newest ledger entry: %+v", ledger[0])
	}

	if err := repo.Withdraw(cl.ID, "char-1", 30, "repair"); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	balance, _ = repo.GetBankBalance(cl.ID)
	if balance != 120 {
		t.Fatalf("balance after withdraw: %d", balance)
	}

	if err := repo.Withdraw(cl.ID, "char-1", 1000, "too much"); err == nil {
		t.Fatal("expected overdraft to fail")
	}
	if err := repo.Deposit(cl.ID, "char-1", 0, "zero"); err == nil {
		t.Fatal("expected zero deposit to fail")
	}
}

func TestClanLevelAndXP(t *testing.T) {
	repo := newSocialTestFactory(t).Clans()
	cl, _ := repo.Store(&clan.Clan{Name: "XP", Tag: "XPX", Active: true})

	if err := repo.AddXP(cl.ID, 500); err != nil {
		t.Fatalf("add xp: %v", err)
	}
	if err := repo.UpdateLevel(cl.ID, 3); err != nil {
		t.Fatalf("update level: %v", err)
	}
	got, _ := repo.FindByID(cl.ID)
	if got.XP != 500 || got.Level != 3 {
		t.Fatalf("unexpected clan progression: xp=%d level=%d", got.XP, got.Level)
	}
}

// ensure the alias keeps the Phase 4 schema naming available.
var _ ClanRepository = (*sqliteClanRepository)(nil)
