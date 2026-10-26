//go:build integration

package repository_test

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"

	"secunda/internal/config"
	"secunda/internal/platform/mysql"
	"secunda/internal/platform/transaction"
	"secunda/internal/repository"
	"secunda/internal/service"
)

func TestTransactionRollsBackRepositories(t *testing.T) {
	_ = godotenv.Load("../../.env")
	cfg, err := config.LoadMySQL()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := mysql.Open(ctx, cfg)
	require.NoError(t, err)
	defer db.Close()
	users := repository.NewUsers(db)
	teams := repository.NewTeams(db)
	manager := transaction.New(db)

	for _, mode := range []string{"callback_error", "foreign_key_error", "panic"} {
		t.Run(mode, func(t *testing.T) {
			email := "rollback-" + rand.Text() + "@example.com"
			failure := errors.New("rollback requested")
			var teamID int64
			action := func() error {
				return manager.WithinTransaction(ctx, func(txCtx context.Context) error {
					user, err := users.Create(txCtx, service.UserCredentials{
						User: service.User{Email: email, Name: "Rollback test", CreatedAt: time.Now().UTC()}, PasswordHash: "unused-test-hash",
					})
					if err != nil {
						return err
					}
					team, err := teams.Create(txCtx, service.Team{Name: "Rollback test", CreatedBy: user.ID, CreatedAt: time.Now().UTC()})
					if err != nil {
						return err
					}
					teamID = team.ID
					if mode == "foreign_key_error" {
						return teams.AddMember(txCtx, service.Membership{TeamID: team.ID, UserID: -1, Role: service.RoleOwner})
					}
					if err := teams.AddMember(txCtx, service.Membership{TeamID: team.ID, UserID: user.ID, Role: service.RoleOwner}); err != nil {
						return err
					}
					if mode == "panic" {
						panic(failure)
					}
					return failure
				})
			}
			if mode == "panic" {
				require.PanicsWithValue(t, failure, func() { _ = action() })
			} else {
				err := action()
				require.Error(t, err)
				if mode == "callback_error" {
					require.ErrorIs(t, err, failure)
				}
			}
			require.Positive(t, teamID, "the callback must create the team before failing")
			_, err := users.FindByEmail(ctx, email)
			require.ErrorIs(t, err, service.ErrNotFound)
			var count int
			require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM teams WHERE id = ?`, teamID).Scan(&count))
			require.Zero(t, count)
			require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_members WHERE team_id = ?`, teamID).Scan(&count))
			require.Zero(t, count)
		})
	}
}
