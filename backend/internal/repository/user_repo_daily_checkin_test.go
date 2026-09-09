package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestClaimDailyCheckIn_CreditsOnlyInsertedRecord(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	checkedInAt := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`WITH inserted AS`).
		WithArgs(int64(42), "2026-07-31", 0.2).
		WillReturnRows(sqlmock.NewRows([]string{"newly_claimed", "reward", "balance", "created_at"}).
			AddRow(true, 0.2, 4.7, checkedInAt))

	result, err := repo.ClaimDailyCheckIn(context.Background(), 42, "2026-07-31", 0.2)
	require.NoError(t, err)
	require.True(t, result.NewlyClaimed)
	require.True(t, result.ClaimedToday)
	require.Equal(t, 0.2, result.Reward)
	require.Equal(t, 4.7, result.Balance)
	require.Equal(t, "2026-07-31", result.CheckInDate)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimDailyCheckIn_RepeatedClaimReturnsCurrentStatus(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	checkedInAt := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`WITH inserted AS`).
		WithArgs(int64(42), "2026-07-31", 0.2).
		WillReturnRows(sqlmock.NewRows([]string{"newly_claimed", "reward", "balance", "created_at"}).
			AddRow(false, 0.2, 4.7, checkedInAt))

	result, err := repo.ClaimDailyCheckIn(context.Background(), 42, "2026-07-31", 0.2)
	require.NoError(t, err)
	require.False(t, result.NewlyClaimed)
	require.True(t, result.ClaimedToday)
	require.Equal(t, 4.7, result.Balance)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListDailyCheckInDatesReturnsOrderedDates(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	mock.ExpectQuery(`SELECT d\.checkin_date::text`).
		WithArgs(int64(42), "2026-08-01", "2026-09-01").
		WillReturnRows(sqlmock.NewRows([]string{"checkin_date"}).
			AddRow("2026-08-01").
			AddRow("2026-08-09"))

	result, err := repo.ListDailyCheckInDates(context.Background(), 42, "2026-08-01", "2026-09-01")
	require.NoError(t, err)
	require.Equal(t, []string{"2026-08-01", "2026-08-09"}, result)
	require.NoError(t, mock.ExpectationsWereMet())
}
