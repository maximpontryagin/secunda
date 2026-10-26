package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mocks "secunda/internal/mocks/service"
	"secunda/internal/service"
)

func TestStatisticsPermissions(t *testing.T) {
	for _, tc := range []struct {
		name               string
		role               service.Role
		lookupErr, wantErr error
	}{
		{name: "owner", role: service.RoleOwner},
		{name: "admin", role: service.RoleAdmin},
		{name: "member", role: service.RoleMember, wantErr: service.ErrForbidden},
		{name: "outsider", lookupErr: service.ErrNotFound, wantErr: service.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			teams := mocks.NewTeamRepository(t)
			statistics := mocks.NewStatisticsRepository(t)
			teams.EXPECT().MemberRole(mock.Anything, int64(10), int64(2)).Return(tc.role, tc.lookupErr).Once()
			if tc.wantErr == nil {
				statistics.EXPECT().Get(mock.Anything, int64(10), mock.Anything).Return(service.Statistics{TeamID: 10}, nil).Once()
			}
			result, err := service.NewReports(statistics, teams).Get(context.Background(), 2, 10)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Zero(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(10), result.TeamID)
			}
		})
	}
}
