package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/talesmud/talesmud/pkg/entities/clan"
	r "github.com/talesmud/talesmud/pkg/repository"
)

// ClansService delivers logical functions on top of the clans repository.
type ClansService interface {
	// Basic operations
	GetClanByID(id string) (*clan.Clan, error)
	GetClans() ([]*clan.Clan, error)
	GetActiveClans() ([]*clan.Clan, error)
	CreateClan(actorCharID, name, tag, description string) (*clan.Clan, error)
	UpdateClan(id string, cl *clan.Clan) error
	DeleteClanByID(id string) error
	FindAll() ([]*clan.Clan, error)
	Store(cl *clan.Clan) (*clan.Clan, error)
	Drop() error

	// Clan for character
	GetClanForCharacter(characterID string) (*clan.Clan, error)
	IsCharacterInClan(characterID string) (bool, error)

	// Membership & invites
	Invite(clanID, actorCharID, targetCharID string) (*clan.ClanInvite, error)
	AcceptInvite(inviteID, characterID string) error
	DeclineInvite(inviteID, characterID string) error
	Kick(clanID, actorCharID, targetCharID string) error
	ChangeRank(clanID, actorCharID, targetCharID string, rank clan.Rank) error
	ListMembers(clanID string) ([]*clan.ClanMember, error)
	GetInvitesForCharacter(characterID string) ([]*clan.ClanInvite, error)
	GetPendingInvitesForCharacter(characterID string) ([]*clan.ClanInvite, error)

	// Bank
	Deposit(clanID, actorCharID string, amount int64) error
	Withdraw(clanID, actorCharID string, amount int64) error
	GetBankBalance(clanID string) (int64, error)
	GetBankLedger(clanID string, limit int) ([]*clan.ClanBankLedger, error)

	// Disband
	Disband(clanID, actorCharID string) error
}

// ParseClanRank converts the lower-case wire name of a rank into a clan.Rank.
// Valid names are "member", "officer" and "leader".
func ParseClanRank(name string) (clan.Rank, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "member":
		return clan.RankMember, nil
	case "officer":
		return clan.RankOfficer, nil
	case "leader":
		return clan.RankLeader, nil
	default:
		return 0, fmt.Errorf("invalid rank %q (expected member, officer or leader)", name)
	}
}

type clansService struct {
	repo r.ClanRepository
}

// NewClansService creates a new clans service.
func NewClansService(clansRepo r.ClanRepository) ClansService {
	return &clansService{
		repo: clansRepo,
	}
}

func (s *clansService) GetClanByID(id string) (*clan.Clan, error) {
	if id == "" {
		return nil, errors.New("clan id is empty")
	}
	return s.repo.FindByID(id)
}

func (s *clansService) GetClans() ([]*clan.Clan, error) {
	return s.repo.FindAll()
}

func (s *clansService) GetActiveClans() ([]*clan.Clan, error) {
	return s.repo.FindAllActive()
}

func (s *clansService) CreateClan(actorCharID, name, tag, description string) (*clan.Clan, error) {
	if actorCharID == "" {
		return nil, errors.New("actor character id is required")
	}
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("clan name is required")
	}
	if strings.TrimSpace(tag) == "" {
		return nil, errors.New("clan tag is required")
	}

	inClan, err := s.repo.IsCharacterInClan(actorCharID)
	if err != nil {
		return nil, err
	}
	if inClan {
		return nil, errors.New("character is already in a clan")
	}

	cl := &clan.Clan{
		Name:         name,
		Tag:          tag,
		Description:  description,
		LeaderCharID: actorCharID,
		Active:       true,
		MaxMembers:   50,
		Level:        1,
		XP:           0,
		Bank:         0,
	}

	created, err := s.repo.Store(cl)
	if err != nil {
		return nil, err
	}

	// Add the founder as a member with leader rank.
	member := &clan.ClanMember{
		CharacterID:  actorCharID,
		ClanID:       created.ID,
		Rank:         clan.RankLeader,
		Contribution: 0,
	}
	if err := s.repo.AddMember(member); err != nil {
		// Attempt to clean up the clan if the membership could not be created.
		_ = s.repo.Delete(created.ID)
		return nil, err
	}

	return created, nil
}

func (s *clansService) UpdateClan(id string, cl *clan.Clan) error {
	if id == "" || cl == nil {
		return errors.New("invalid clan update parameters")
	}
	return s.repo.Update(id, cl)
}

func (s *clansService) DeleteClanByID(id string) error {
	return s.repo.Delete(id)
}

func (s *clansService) FindAll() ([]*clan.Clan, error) {
	return s.repo.FindAll()
}

func (s *clansService) Store(cl *clan.Clan) (*clan.Clan, error) {
	if cl == nil {
		return nil, errors.New("clan is nil")
	}
	return s.repo.Store(cl)
}

func (s *clansService) Drop() error {
	return s.repo.Drop()
}

func (s *clansService) GetClanForCharacter(characterID string) (*clan.Clan, error) {
	return s.repo.GetCharacterClan(characterID)
}

func (s *clansService) IsCharacterInClan(characterID string) (bool, error) {
	return s.repo.IsCharacterInClan(characterID)
}

// requireMember loads a membership and normalises the "not found" repository
// error into a clear authorization failure.
func (s *clansService) requireMember(clanID, characterID string) (*clan.ClanMember, error) {
	member, err := s.repo.FindMember(clanID, characterID)
	if err != nil || member == nil {
		return nil, errors.New("actor is not a member of this clan")
	}
	return member, nil
}

// Membership & invites

func (s *clansService) Invite(clanID, actorCharID, targetCharID string) (*clan.ClanInvite, error) {
	if clanID == "" || actorCharID == "" || targetCharID == "" {
		return nil, errors.New("invalid invite parameters")
	}
	if actorCharID == targetCharID {
		return nil, errors.New("cannot invite yourself")
	}

	cl, err := s.repo.FindByID(clanID)
	if err != nil {
		return nil, err
	}
	if cl == nil || !cl.Active {
		return nil, errors.New("clan not found or not active")
	}

	actor, err := s.requireMember(clanID, actorCharID)
	if err != nil {
		return nil, err
	}
	// Only leader or officer can invite.
	if actor.Rank != clan.RankLeader && actor.Rank != clan.RankOfficer {
		return nil, errors.New("insufficient rank to invite")
	}

	// Target must not already belong to a clan.
	inClan, err := s.repo.IsCharacterInClan(targetCharID)
	if err != nil {
		return nil, err
	}
	if inClan {
		return nil, errors.New("target is already in a clan")
	}

	// Avoid duplicate pending invites for the same (clan, character).
	existing, err := s.repo.FindInvite(clanID, targetCharID)
	if err == nil && existing != nil && existing.Status == clan.InviteStatusPending {
		return nil, errors.New("pending invite already exists")
	}

	invite := &clan.ClanInvite{
		ClanID:      clanID,
		CharacterID: targetCharID,
		InviterID:   actorCharID,
		Status:      clan.InviteStatusPending,
	}
	return s.repo.CreateInvite(invite)
}

// findPendingInviteByID locates one of the character's own pending invites.
// The repository keys invites by (clanID, characterID) but the HTTP API exposes
// the invite's UUID, so the character's invite list is searched by ID.
func (s *clansService) findPendingInviteByID(inviteID, characterID string) (*clan.ClanInvite, error) {
	if inviteID == "" || characterID == "" {
		return nil, errors.New("invalid invite parameters")
	}
	invites, err := s.repo.FindInvitesByCharacter(characterID)
	if err != nil {
		return nil, err
	}
	for _, inv := range invites {
		if inv != nil && inv.ID == inviteID && inv.Status == clan.InviteStatusPending {
			return inv, nil
		}
	}
	return nil, errors.New("pending invite not found")
}

func (s *clansService) AcceptInvite(inviteID, characterID string) error {
	target, err := s.findPendingInviteByID(inviteID, characterID)
	if err != nil {
		return err
	}

	inClan, err := s.repo.IsCharacterInClan(characterID)
	if err != nil {
		return err
	}
	if inClan {
		return errors.New("character already in a clan")
	}

	member := &clan.ClanMember{
		CharacterID:  characterID,
		ClanID:       target.ClanID,
		Rank:         clan.RankMember,
		Contribution: 0,
	}
	if err := s.repo.AddMember(member); err != nil {
		return err
	}

	return s.repo.UpdateInviteStatus(target.ClanID, target.CharacterID, clan.InviteStatusAccepted)
}

func (s *clansService) DeclineInvite(inviteID, characterID string) error {
	target, err := s.findPendingInviteByID(inviteID, characterID)
	if err != nil {
		return err
	}
	return s.repo.UpdateInviteStatus(target.ClanID, target.CharacterID, clan.InviteStatusDeclined)
}

func (s *clansService) Kick(clanID, actorCharID, targetCharID string) error {
	if clanID == "" || actorCharID == "" || targetCharID == "" {
		return errors.New("invalid kick parameters")
	}
	if actorCharID == targetCharID {
		return errors.New("cannot kick yourself")
	}

	if _, err := s.repo.FindByID(clanID); err != nil {
		return err
	}

	actor, err := s.requireMember(clanID, actorCharID)
	if err != nil {
		return err
	}
	target, err := s.repo.FindMember(clanID, targetCharID)
	if err != nil || target == nil {
		return errors.New("target character is not a member of this clan")
	}

	// Only the leader or an officer may kick.
	if actor.Rank != clan.RankLeader && actor.Rank != clan.RankOfficer {
		return errors.New("insufficient rank to kick")
	}
	// The leader can never be removed by someone else.
	if target.Rank == clan.RankLeader {
		return errors.New("cannot kick the clan leader")
	}
	return s.repo.RemoveMember(clanID, targetCharID)
}

func (s *clansService) ChangeRank(clanID, actorCharID, targetCharID string, rank clan.Rank) error {
	if clanID == "" || actorCharID == "" || targetCharID == "" {
		return errors.New("invalid rank change parameters")
	}
	switch rank {
	case clan.RankMember, clan.RankOfficer, clan.RankLeader:
	default:
		return errors.New("invalid rank")
	}

	actor, err := s.requireMember(clanID, actorCharID)
	if err != nil {
		return err
	}
	if _, err := s.repo.FindMember(clanID, targetCharID); err != nil {
		return errors.New("target character is not a member of this clan")
	}

	// Only the leader can change ranks.
	if actor.Rank != clan.RankLeader {
		return errors.New("only the leader can change ranks")
	}
	if actorCharID == targetCharID && rank != clan.RankLeader {
		return errors.New("the leader cannot demote themselves")
	}
	return s.repo.UpdateMemberRank(clanID, targetCharID, rank)
}

func (s *clansService) ListMembers(clanID string) ([]*clan.ClanMember, error) {
	if clanID == "" {
		return nil, errors.New("clan id is empty")
	}
	return s.repo.FindMembersByClan(clanID)
}

func (s *clansService) GetInvitesForCharacter(characterID string) ([]*clan.ClanInvite, error) {
	return s.repo.FindInvitesByCharacter(characterID)
}

func (s *clansService) GetPendingInvitesForCharacter(characterID string) ([]*clan.ClanInvite, error) {
	invites, err := s.repo.FindInvitesByCharacter(characterID)
	if err != nil {
		return nil, err
	}
	pending := make([]*clan.ClanInvite, 0, len(invites))
	for _, inv := range invites {
		if inv != nil && inv.Status == clan.InviteStatusPending {
			pending = append(pending, inv)
		}
	}
	return pending, nil
}

func (s *clansService) Deposit(clanID, actorCharID string, amount int64) error {
	if clanID == "" || actorCharID == "" {
		return errors.New("invalid deposit parameters")
	}
	if amount <= 0 {
		return errors.New("deposit amount must be positive")
	}
	if _, err := s.requireMember(clanID, actorCharID); err != nil {
		return err
	}
	return s.repo.Deposit(clanID, actorCharID, amount, "deposit")
}

func (s *clansService) Withdraw(clanID, actorCharID string, amount int64) error {
	if clanID == "" || actorCharID == "" {
		return errors.New("invalid withdraw parameters")
	}
	if amount <= 0 {
		return errors.New("withdraw amount must be positive")
	}
	member, err := s.requireMember(clanID, actorCharID)
	if err != nil {
		return err
	}
	// Only the leader can withdraw from the clan bank.
	if member.Rank != clan.RankLeader {
		return errors.New("only the leader can withdraw")
	}
	return s.repo.Withdraw(clanID, actorCharID, amount, "withdraw")
}

func (s *clansService) GetBankBalance(clanID string) (int64, error) {
	return s.repo.GetBankBalance(clanID)
}

func (s *clansService) GetBankLedger(clanID string, limit int) ([]*clan.ClanBankLedger, error) {
	return s.repo.GetBankLedger(clanID, limit)
}

func (s *clansService) Disband(clanID, actorCharID string) error {
	if clanID == "" || actorCharID == "" {
		return errors.New("invalid disband parameters")
	}

	if _, err := s.repo.FindByID(clanID); err != nil {
		return err
	}

	member, err := s.requireMember(clanID, actorCharID)
	if err != nil {
		return err
	}
	if member.Rank != clan.RankLeader {
		return errors.New("only the leader can disband the clan")
	}

	members, err := s.repo.FindMembersByClan(clanID)
	if err != nil {
		return err
	}
	for _, m := range members {
		if m != nil {
			_ = s.repo.RemoveMember(clanID, m.CharacterID)
		}
	}
	return s.repo.Delete(clanID)
}
