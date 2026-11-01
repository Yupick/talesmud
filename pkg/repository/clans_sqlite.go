package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbpkg "github.com/talesmud/talesmud/pkg/db"
	dbsqlite "github.com/talesmud/talesmud/pkg/db/sqlite"
	e "github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/clan"
)

type sqliteClanRepository struct {
	*sqliteGenericRepo

	members *sqliteGenericRepo
	invites *sqliteGenericRepo
	ledger  *sqliteGenericRepo

	db *sql.DB
}

// NewSQLiteClanRepository creates a new SQLite clan repository.
func NewSQLiteClanRepository(client *dbsqlite.Client) ClanRepository {
	conn := client.DB()
	return &sqliteClanRepository{
		sqliteGenericRepo: newSQLiteGenericRepo(conn, "clans", func() interface{} {
			return &clan.Clan{}
		}),
		members: newSQLiteGenericRepo(conn, "clan_members", func() interface{} {
			return &clan.ClanMember{}
		}),
		invites: newSQLiteGenericRepo(conn, "clan_invites", func() interface{} {
			return &clan.ClanInvite{}
		}),
		ledger: newSQLiteGenericRepo(conn, "clan_bank_ledger", func() interface{} {
			return &clan.ClanBankLedger{}
		}),
		db: conn,
	}
}

// collectRepo runs a (possibly filtered) scan over a generic table and returns
// the decoded entities, typed via the entity pointer.
func collectRepo[T any](repo *sqliteGenericRepo, params *dbpkg.QueryParams) ([]*T, error) {
	out := make([]*T, 0)
	collector := func(elem interface{}) {
		if typed, ok := elem.(*T); ok {
			out = append(out, typed)
		}
	}
	var err error
	if params == nil {
		err = repo.FindAll(collector)
	} else {
		err = repo.FindAllWithParam(params, collector)
	}
	return out, err
}

// findOneRepo returns the first entity matching the given params.
func findOneRepo[T any](repo *sqliteGenericRepo, params *dbpkg.QueryParams) (*T, error) {
	results, err := collectRepo[T](repo, params)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, errors.New("entity not found")
	}
	return results[0], nil
}

// ---------- Clans CRUD ----------

func (repo *sqliteClanRepository) Drop() error {
	// Clear all clan related collections so tests and operators get a clean slate.
	for _, table := range []string{"clans", "clan_members", "clan_invites", "clan_bank_ledger"} {
		if _, err := repo.db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			return err
		}
	}
	return nil
}

func (repo *sqliteClanRepository) FindByID(id string) (*clan.Clan, error) {
	result, err := repo.sqliteGenericRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	return result.(*clan.Clan), nil
}

func (repo *sqliteClanRepository) FindByName(name string) ([]*clan.Clan, error) {
	return collectRepo[clan.Clan](repo.sqliteGenericRepo, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "name", Value: name},
	))
}

func (repo *sqliteClanRepository) FindByTag(tag string) ([]*clan.Clan, error) {
	return collectRepo[clan.Clan](repo.sqliteGenericRepo, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "tag", Value: tag},
	))
}

func (repo *sqliteClanRepository) FindByLeader(leaderCharID string) ([]*clan.Clan, error) {
	return collectRepo[clan.Clan](repo.sqliteGenericRepo, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "leaderCharID", Value: leaderCharID},
	))
}

func (repo *sqliteClanRepository) FindAll() ([]*clan.Clan, error) {
	return collectRepo[clan.Clan](repo.sqliteGenericRepo, nil)
}

func (repo *sqliteClanRepository) FindAllActive() ([]*clan.Clan, error) {
	return collectRepo[clan.Clan](repo.sqliteGenericRepo, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "active", Value: 1},
	))
}

func (repo *sqliteClanRepository) Store(cl *clan.Clan) (*clan.Clan, error) {
	if cl == nil {
		return nil, errors.New("clan is nil")
	}
	if cl.Entity == nil {
		cl.Entity = e.NewEntity()
	}
	now := time.Now()
	if cl.CreatedAt.IsZero() {
		cl.CreatedAt = now
	}
	if cl.FoundedAt.IsZero() {
		cl.FoundedAt = cl.CreatedAt
	}
	cl.UpdatedAt = now
	result, err := repo.sqliteGenericRepo.Store(cl)
	if err != nil {
		return nil, err
	}
	return result.(*clan.Clan), nil
}

func (repo *sqliteClanRepository) Update(id string, cl *clan.Clan) error {
	if cl == nil {
		return errors.New("clan is nil")
	}
	cl.UpdatedAt = time.Now()
	return repo.sqliteGenericRepo.Update(cl, id)
}

func (repo *sqliteClanRepository) Delete(id string) error {
	return repo.sqliteGenericRepo.Delete(id)
}

// ---------- Membership ----------

func (repo *sqliteClanRepository) AddMember(m *clan.ClanMember) error {
	if m == nil {
		return errors.New("member is nil")
	}
	if m.CharacterID == "" || m.ClanID == "" {
		return errors.New("member requires characterID and clanID")
	}
	// A character may only belong to one clan at a time.
	existing, err := repo.FindMembersByCharacter(m.CharacterID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("character %s already belongs to clan %s", m.CharacterID, existing[0].ClanID)
	}
	if m.Entity == nil {
		m.Entity = e.NewEntity()
	}
	if m.JoinedAt.IsZero() {
		m.JoinedAt = time.Now()
	}
	_, err = repo.members.Store(m)
	return err
}

func (repo *sqliteClanRepository) RemoveMember(clanID, characterID string) error {
	member, err := repo.FindMember(clanID, characterID)
	if err != nil {
		return err
	}
	return repo.members.Delete(member.ID)
}

func (repo *sqliteClanRepository) UpdateMemberRank(clanID, characterID string, rank clan.Rank) error {
	member, err := repo.FindMember(clanID, characterID)
	if err != nil {
		return err
	}
	member.Rank = rank
	return repo.members.Update(member, member.ID)
}

func (repo *sqliteClanRepository) FindMember(clanID, characterID string) (*clan.ClanMember, error) {
	return findOneRepo[clan.ClanMember](repo.members, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "clanID", Value: clanID},
		dbpkg.QueryParam{Key: "characterID", Value: characterID},
	))
}

func (repo *sqliteClanRepository) FindMembersByClan(clanID string) ([]*clan.ClanMember, error) {
	return collectRepo[clan.ClanMember](repo.members, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "clanID", Value: clanID},
	))
}

func (repo *sqliteClanRepository) FindMembersByCharacter(characterID string) ([]*clan.ClanMember, error) {
	return collectRepo[clan.ClanMember](repo.members, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "characterID", Value: characterID},
	))
}

func (repo *sqliteClanRepository) UpdateMemberContribution(clanID, characterID string, amount int64) error {
	member, err := repo.FindMember(clanID, characterID)
	if err != nil {
		return err
	}
	member.Contribution += amount
	return repo.members.Update(member, member.ID)
}

// ---------- Invites ----------

func (repo *sqliteClanRepository) CreateInvite(inv *clan.ClanInvite) (*clan.ClanInvite, error) {
	if inv == nil {
		return nil, errors.New("invite is nil")
	}
	if inv.ClanID == "" || inv.CharacterID == "" {
		return nil, errors.New("invite requires clanID and characterID")
	}
	if inv.Entity == nil {
		inv.Entity = e.NewEntity()
	}
	now := time.Now()
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = now
	}
	if inv.ExpiresAt.IsZero() {
		inv.ExpiresAt = inv.CreatedAt.Add(7 * 24 * time.Hour)
	}
	if inv.Status == "" {
		inv.Status = clan.InviteStatusPending
	}
	result, err := repo.invites.Store(inv)
	if err != nil {
		return nil, err
	}
	return result.(*clan.ClanInvite), nil
}

func (repo *sqliteClanRepository) FindInvite(clanID, characterID string) (*clan.ClanInvite, error) {
	return findOneRepo[clan.ClanInvite](repo.invites, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "clanID", Value: clanID},
		dbpkg.QueryParam{Key: "characterID", Value: characterID},
	))
}

func (repo *sqliteClanRepository) FindInvitesByClan(clanID string) ([]*clan.ClanInvite, error) {
	return collectRepo[clan.ClanInvite](repo.invites, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "clanID", Value: clanID},
	))
}

func (repo *sqliteClanRepository) FindInvitesByCharacter(characterID string) ([]*clan.ClanInvite, error) {
	return collectRepo[clan.ClanInvite](repo.invites, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "characterID", Value: characterID},
	))
}

func (repo *sqliteClanRepository) UpdateInviteStatus(clanID, characterID string, status string) error {
	invite, err := repo.FindInvite(clanID, characterID)
	if err != nil {
		return err
	}
	invite.Status = status
	return repo.invites.Update(invite, invite.ID)
}

func (repo *sqliteClanRepository) DeleteInvite(clanID, characterID string) error {
	invite, err := repo.FindInvite(clanID, characterID)
	if err != nil {
		return err
	}
	return repo.invites.Delete(invite.ID)
}

// ---------- Clan bank ----------

func (repo *sqliteClanRepository) GetBankBalance(clanID string) (int64, error) {
	cl, err := repo.FindByID(clanID)
	if err != nil {
		return 0, err
	}
	return cl.Bank, nil
}

func (repo *sqliteClanRepository) Deposit(clanID, characterID string, amount int64, note string) error {
	if amount <= 0 {
		return errors.New("deposit amount must be positive")
	}
	cl, err := repo.FindByID(clanID)
	if err != nil {
		return err
	}
	cl.Bank += amount
	if err := repo.Update(clanID, cl); err != nil {
		return err
	}
	return repo.appendLedger(&clan.ClanBankLedger{
		ClanID:      clanID,
		CharacterID: characterID,
		Type:        clan.BankTypeDeposit,
		Amount:      amount,
		Balance:     cl.Bank,
		Note:        note,
		Timestamp:   time.Now(),
	})
}

func (repo *sqliteClanRepository) Withdraw(clanID, characterID string, amount int64, note string) error {
	if amount <= 0 {
		return errors.New("withdraw amount must be positive")
	}
	cl, err := repo.FindByID(clanID)
	if err != nil {
		return err
	}
	if cl.Bank < amount {
		return fmt.Errorf("insufficient clan bank balance: have %d, need %d", cl.Bank, amount)
	}
	cl.Bank -= amount
	if err := repo.Update(clanID, cl); err != nil {
		return err
	}
	return repo.appendLedger(&clan.ClanBankLedger{
		ClanID:      clanID,
		CharacterID: characterID,
		Type:        clan.BankTypeWithdraw,
		Amount:      amount,
		Balance:     cl.Bank,
		Note:        note,
		Timestamp:   time.Now(),
	})
}

// appendLedger writes an append-only bank ledger entry.
func (repo *sqliteClanRepository) appendLedger(entry *clan.ClanBankLedger) error {
	if entry.Entity == nil {
		entry.Entity = e.NewEntity()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	_, err := repo.ledger.Store(entry)
	return err
}

func (repo *sqliteClanRepository) GetBankLedger(clanID string, limit int) ([]*clan.ClanBankLedger, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := repo.db.Query(
		`SELECT data FROM clan_bank_ledger
		 WHERE json_extract(data, '$.clanID') = ?
		 ORDER BY json_extract(data, '$.timestamp') DESC
		 LIMIT ?`,
		clanID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*clan.ClanBankLedger, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		entry := &clan.ClanBankLedger{}
		if err := json.Unmarshal([]byte(payload), entry); err != nil {
			continue
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// ---------- Level and XP ----------

func (repo *sqliteClanRepository) AddXP(clanID string, amount int64) error {
	cl, err := repo.FindByID(clanID)
	if err != nil {
		return err
	}
	cl.XP += amount
	return repo.Update(clanID, cl)
}

func (repo *sqliteClanRepository) UpdateLevel(clanID string, level int) error {
	cl, err := repo.FindByID(clanID)
	if err != nil {
		return err
	}
	cl.Level = level
	return repo.Update(clanID, cl)
}

// ---------- Helpers ----------

func (repo *sqliteClanRepository) IsCharacterInClan(characterID string) (bool, error) {
	memberships, err := repo.FindMembersByCharacter(characterID)
	if err != nil {
		return false, err
	}
	return len(memberships) > 0, nil
}

func (repo *sqliteClanRepository) GetCharacterClan(characterID string) (*clan.Clan, error) {
	memberships, err := repo.FindMembersByCharacter(characterID)
	if err != nil {
		return nil, err
	}
	if len(memberships) == 0 {
		return nil, fmt.Errorf("character %s is not in a clan", characterID)
	}
	return repo.FindByID(memberships[0].ClanID)
}
