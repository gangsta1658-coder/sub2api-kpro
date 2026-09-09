//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type dailyCheckInUserRepo struct {
	*mockUserRepo
	getResult     DailyCheckIn
	claimResult   DailyCheckIn
	getDate       string
	claimDate     string
	claimReward   float64
	calendarDates []string
	calendarFrom  string
	calendarTo    string
}

func (r *dailyCheckInUserRepo) GetDailyCheckIn(_ context.Context, _ int64, checkInDate string) (DailyCheckIn, error) {
	r.getDate = checkInDate
	return r.getResult, nil
}

func (r *dailyCheckInUserRepo) ClaimDailyCheckIn(_ context.Context, _ int64, checkInDate string, reward float64) (DailyCheckIn, error) {
	r.claimDate = checkInDate
	r.claimReward = reward
	return r.claimResult, nil
}

func (r *dailyCheckInUserRepo) ListDailyCheckInDates(_ context.Context, _ int64, fromDate, toDate string) ([]string, error) {
	r.calendarFrom = fromDate
	r.calendarTo = toDate
	return r.calendarDates, nil
}

func TestGetDailyCheckIn_ExposesRewardBeforeClaim(t *testing.T) {
	repo := &dailyCheckInUserRepo{mockUserRepo: &mockUserRepo{}, getResult: DailyCheckIn{Balance: 3.5}}
	svc := NewUserService(repo, nil, nil, nil)

	result, err := svc.GetDailyCheckIn(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, DailyCheckInReward, result.Reward)
	require.NotEmpty(t, repo.getDate)
}

func TestClaimDailyCheckIn_UsesFixedReward(t *testing.T) {
	repo := &dailyCheckInUserRepo{
		mockUserRepo: &mockUserRepo{},
		claimResult:  DailyCheckIn{ClaimedToday: true, NewlyClaimed: true, Reward: DailyCheckInReward, Balance: 3.7},
	}
	svc := NewUserService(repo, nil, nil, nil)

	result, err := svc.ClaimDailyCheckIn(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, result.NewlyClaimed)
	require.Equal(t, DailyCheckInReward, repo.claimReward)
	require.NotEmpty(t, repo.claimDate)
}

func TestGetDailyCheckInCalendarUsesCurrentServerMonth(t *testing.T) {
	repo := &dailyCheckInUserRepo{
		mockUserRepo:  &mockUserRepo{},
		calendarDates: []string{"2026-08-01", "2026-08-09"},
	}
	svc := NewUserService(repo, nil, nil, nil)

	result, err := svc.GetDailyCheckInCalendar(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, repo.calendarDates, result.CheckInDates)
	require.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, result.CheckInDate)
	require.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, repo.calendarFrom)
	require.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, repo.calendarTo)
}
