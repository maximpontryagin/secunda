package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

type TeamRepository interface {
	Create(context.Context, Team) (Team, error)
	AddMember(context.Context, Membership) error
	ListByUser(context.Context, int64) ([]Team, error)
	MemberRole(context.Context, int64, int64) (Role, error)
}

type Teams struct {
	teams        TeamRepository
	users        UserRepository
	transactions TransactionManager
}

func NewTeams(teams TeamRepository, users UserRepository, transactions TransactionManager) *Teams {
	return &Teams{teams: teams, users: users, transactions: transactions}
}

func (s *Teams) Create(ctx context.Context, actorID int64, input CreateTeamInput) (Team, error) {
	if actorID <= 0 {
		return Team{}, ErrUnauthorized
	}
	name := strings.TrimSpace(input.Name)
	if !validName(name) {
		return Team{}, invalid("name must contain between 1 and 255 characters")
	}
	var team Team
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		team, err = s.teams.Create(txCtx, Team{
			Name: name, CreatedBy: actorID, CreatedAt: time.Now().UTC().Truncate(time.Microsecond), Role: RoleOwner,
		})
		if err != nil {
			return err
		}
		return s.teams.AddMember(txCtx, Membership{TeamID: team.ID, UserID: actorID, Role: RoleOwner})
	})
	if err != nil {
		return Team{}, err
	}
	return team, nil
}

func (s *Teams) List(ctx context.Context, actorID int64) ([]Team, error) {
	if actorID <= 0 {
		return nil, ErrUnauthorized
	}
	return s.teams.ListByUser(ctx, actorID)
}

func (s *Teams) Invite(ctx context.Context, actorID int64, input InviteInput) (Membership, error) {
	if actorID <= 0 {
		return Membership{}, ErrUnauthorized
	}
	if input.TeamID <= 0 {
		return Membership{}, invalid("team id must be positive")
	}
	actorRole, err := s.teams.MemberRole(ctx, input.TeamID, actorID)
	if errors.Is(err, ErrNotFound) {
		return Membership{}, ErrNotFound
	}
	if err != nil {
		return Membership{}, err
	}
	if actorRole != RoleOwner && actorRole != RoleAdmin {
		return Membership{}, ErrForbidden
	}
	role := input.Role
	if role == "" {
		role = RoleMember
	}
	if role != RoleMember && role != RoleAdmin {
		return Membership{}, invalid("role must be admin or member")
	}
	if actorRole == RoleAdmin && role != RoleMember {
		return Membership{}, ErrForbidden
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return Membership{}, err
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return Membership{}, err
	}
	member := Membership{TeamID: input.TeamID, UserID: user.User.ID, Role: role}
	if err := s.teams.AddMember(ctx, member); err != nil {
		return Membership{}, err
	}
	return member, nil
}
