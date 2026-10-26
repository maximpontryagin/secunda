package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mocks "secunda/internal/mocks/service"
	"secunda/internal/service"
)

func TestInvitePermissions(t *testing.T) {
	cases := []struct {
		name          string
		actorRole     service.Role
		requestedRole service.Role
		lookupError   error
		wantError     error
		wantRole      service.Role
	}{
		{name: "owner invites member by default", actorRole: service.RoleOwner, wantRole: service.RoleMember},
		{name: "owner invites admin", actorRole: service.RoleOwner, requestedRole: service.RoleAdmin, wantRole: service.RoleAdmin},
		{name: "admin invites member", actorRole: service.RoleAdmin, requestedRole: service.RoleMember, wantRole: service.RoleMember},
		{name: "admin cannot invite admin", actorRole: service.RoleAdmin, requestedRole: service.RoleAdmin, wantError: service.ErrForbidden},
		{name: "member cannot invite", actorRole: service.RoleMember, wantError: service.ErrForbidden},
		{name: "outsider cannot inspect invitation target", lookupError: service.ErrNotFound, wantError: service.ErrNotFound},
		{name: "owner role cannot be assigned", actorRole: service.RoleOwner, requestedRole: service.RoleOwner, wantError: service.ErrInvalid},
		{name: "unknown role rejected", actorRole: service.RoleOwner, requestedRole: "superuser", wantError: service.ErrInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			teams := mocks.NewTeamRepository(t)
			users := mocks.NewUserRepository(t)
			transactions := mocks.NewTransactionManager(t)
			teams.EXPECT().MemberRole(ctx, int64(10), int64(1)).Return(tc.actorRole, tc.lookupError).Once()
			if tc.wantError == nil {
				users.EXPECT().FindByEmail(ctx, "bob@example.com").Return(service.UserCredentials{User: service.User{ID: 2}}, nil).Once()
				teams.EXPECT().AddMember(ctx, service.Membership{TeamID: 10, UserID: 2, Role: tc.wantRole}).Return(nil).Once()
			}
			result, err := service.NewTeams(teams, users, transactions).Invite(ctx, 1,
				service.InviteInput{TeamID: 10, Email: " BOB@example.com ", Role: tc.requestedRole})
			if tc.wantError != nil {
				require.ErrorIs(t, err, tc.wantError)
				require.Zero(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantRole, result.Role)
			}
		})
	}
}

func TestInviteDuplicateDoesNotChangeRole(t *testing.T) {
	ctx := context.Background()
	teams := mocks.NewTeamRepository(t)
	users := mocks.NewUserRepository(t)
	transactions := mocks.NewTransactionManager(t)
	teams.EXPECT().MemberRole(ctx, int64(10), int64(1)).Return(service.RoleOwner, nil).Once()
	users.EXPECT().FindByEmail(ctx, "bob@example.com").Return(service.UserCredentials{User: service.User{ID: 2}}, nil).Once()
	teams.EXPECT().AddMember(ctx, service.Membership{TeamID: 10, UserID: 2, Role: service.RoleAdmin}).Return(service.ErrConflict).Once()
	_, err := service.NewTeams(teams, users, transactions).Invite(ctx, 1,
		service.InviteInput{TeamID: 10, Email: "bob@example.com", Role: service.RoleAdmin})
	require.ErrorIs(t, err, service.ErrConflict)
}

func TestCreateTeamUsesTransactionContext(t *testing.T) {
	for _, failMember := range []bool{false, true} {
		name := "commit"
		if failMember {
			name = "member insertion failure"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			txCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			teams := mocks.NewTeamRepository(t)
			users := mocks.NewUserRepository(t)
			transactions := mocks.NewTransactionManager(t)
			failure := errors.New("member insert failed")
			var memberError error
			if failMember {
				memberError = failure
			}
			transactions.EXPECT().WithinTransaction(ctx, mock.Anything).RunAndReturn(func(_ context.Context, fn func(context.Context) error) error {
				return fn(txCtx)
			}).Once()
			teams.EXPECT().Create(txCtx, mock.MatchedBy(func(team service.Team) bool {
				return team.Name == "Backend" && team.CreatedBy == 1 && team.Role == service.RoleOwner
			})).Return(service.Team{ID: 10, Name: "Backend", CreatedBy: 1, Role: service.RoleOwner}, nil).Once()
			teams.EXPECT().AddMember(txCtx, service.Membership{TeamID: 10, UserID: 1, Role: service.RoleOwner}).Return(memberError).Once()
			result, err := service.NewTeams(teams, users, transactions).Create(ctx, 1, service.CreateTeamInput{Name: " Backend "})
			if failMember {
				require.ErrorIs(t, err, failure)
				require.Zero(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(10), result.ID)
			}
		})
	}
}

func TestCreateTeamCommitFailure(t *testing.T) {
	teams := mocks.NewTeamRepository(t)
	users := mocks.NewUserRepository(t)
	transactions := mocks.NewTransactionManager(t)
	failure := errors.New("commit failed")
	transactions.EXPECT().WithinTransaction(mock.Anything, mock.Anything).Return(failure).Once()
	result, err := service.NewTeams(teams, users, transactions).Create(context.Background(), 1, service.CreateTeamInput{Name: "Backend"})
	require.ErrorIs(t, err, failure)
	require.Zero(t, result)
}

func TestTeamListUsesCurrentUser(t *testing.T) {
	teams := mocks.NewTeamRepository(t)
	users := mocks.NewUserRepository(t)
	transactions := mocks.NewTransactionManager(t)
	teams.EXPECT().ListByUser(mock.Anything, int64(42)).Return([]service.Team{{ID: 10}}, nil).Once()
	result, err := service.NewTeams(teams, users, transactions).List(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, int64(10), result[0].ID)
}
