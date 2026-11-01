// Package clan holds the entities backing the Phase 4 clan system:
// the clan itself, its membership, pending invites and the bank ledger.
package clan

import (
	"time"

	"github.com/talesmud/talesmud/pkg/entities"
)

// Rank defines a member's rank inside a clan.
type Rank int

const (
	// RankMember is the default rank for a newly joined character.
	RankMember Rank = iota + 1
	// RankOfficer can invite and manage regular members.
	RankOfficer
	// RankLeader owns the clan and can dissolve it.
	RankLeader
)

// String returns the lower-case wire name used in JSON and commands.
func (r Rank) String() string {
	switch r {
	case RankMember:
		return "member"
	case RankOfficer:
		return "officer"
	case RankLeader:
		return "leader"
	default:
		return "unknown"
	}
}

// Invite status values.
const (
	InviteStatusPending  = "pending"
	InviteStatusAccepted = "accepted"
	InviteStatusDeclined = "declined"
	InviteStatusExpired  = "expired"
)

// Bank ledger transaction types.
const (
	BankTypeDeposit  = "deposit"
	BankTypeWithdraw = "withdraw"
)

// Clan is the main clan entity.
type Clan struct {
	*entities.Entity `json:",inline"`

	Name         string    `json:"name"`
	Tag          string    `json:"tag"` // Short tag (e.g. "DRK")
	LeaderCharID string    `json:"leaderCharID"`
	FoundedAt    time.Time `json:"foundedAt"`
	Level        int       `json:"level"`
	XP           int64     `json:"xp"`
	Bank         int64     `json:"bank"` // Clan gold
	MOTD         string    `json:"motd"` // Message of the day
	Description  string    `json:"description"`
	Active       bool      `json:"active"` // false = dissolved
	MaxMembers   int       `json:"maxMembers"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ClanMember is a character's membership in a clan.
type ClanMember struct {
	*entities.Entity `json:",inline"`

	CharacterID  string    `json:"characterID"`
	ClanID       string    `json:"clanID"`
	Rank         Rank      `json:"rank"`
	JoinedAt     time.Time `json:"joinedAt"`
	Contribution int64     `json:"contribution"` // Total contributed to the clan
	LastOnline   time.Time `json:"lastOnline"`
}

// ClanInvite is a pending invitation to join a clan.
type ClanInvite struct {
	*entities.Entity `json:",inline"`

	ClanID      string    `json:"clanID"`
	CharacterID string    `json:"characterID"`
	InviterID   string    `json:"inviterID"` // Who sent the invite
	Status      string    `json:"status"`    // pending | accepted | declined | expired
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// ClanBankLedger records a clan bank transaction (append-only).
type ClanBankLedger struct {
	*entities.Entity `json:",inline"`

	ClanID      string    `json:"clanID"`
	CharacterID string    `json:"characterID"` // Who performed the action
	Type        string    `json:"type"`        // deposit | withdraw
	Amount      int64     `json:"amount"`
	Balance     int64     `json:"balance"` // Balance after the transaction
	Note        string    `json:"note"`    // Optional description
	Timestamp   time.Time `json:"timestamp"`
}

// Doc-compatible aliases. The Phase 4 schema design refers to these entities by
// their short package-qualified names (clan.Member, clan.Invite, ...); the
// repository interfaces also accept the explicit names.
type (
	// Member is an alias for ClanMember.
	Member = ClanMember
	// Invite is an alias for ClanInvite.
	Invite = ClanInvite
	// BankLedgerEntry is an alias for ClanBankLedger.
	BankLedgerEntry = ClanBankLedger
)
