package notificationcenter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/infra/kvstore"
)

func newTestService() *Service {
	svc := ProvideService(kvstore.NewFakeKVStore())
	svc.now = func() time.Time {
		return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	}
	next := 0
	ids := []string{"id-older", "id-newer", "id-other"}
	svc.newID = func() string {
		id := ids[next%len(ids)]
		next++
		return id
	}
	return svc
}

func TestListReturnsNewestFirstForTheCallerOnly(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	older, err := svc.insert(ctx, 1, 7, Notification{Title: "Older", Body: "first"})
	require.NoError(t, err)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	}
	newer, err := svc.insert(ctx, 1, 7, Notification{Title: "Newer", Body: "second"})
	require.NoError(t, err)
	_, err = svc.insert(ctx, 1, 8, Notification{Title: "Someone else", Body: "hidden"})
	require.NoError(t, err)

	got, err := svc.List(ctx, 1, 7)
	require.NoError(t, err)
	require.Equal(t, []Notification{
		{ID: newer.ID, Title: "Newer", Body: "second", Created: time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC), Read: false},
		{ID: older.ID, Title: "Older", Body: "first", Created: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC), Read: false},
	}, got)
}

func TestUnreadCountCountsOnlyTheCallersUnread(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.insert(ctx, 1, 7, Notification{Title: "Unread", Body: "a"})
	require.NoError(t, err)
	read, err := svc.insert(ctx, 1, 7, Notification{Title: "Read", Body: "b", Read: true})
	require.NoError(t, err)
	require.True(t, read.Read)
	_, err = svc.insert(ctx, 1, 9, Notification{Title: "Other user", Body: "c"})
	require.NoError(t, err)

	count, err := svc.UnreadCount(ctx, 1, 7)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	other, err := svc.UnreadCount(ctx, 1, 9)
	require.NoError(t, err)
	require.Equal(t, 1, other)
}

func TestMarkReadDoesNotChangeAnotherUsersNotification(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	mine, err := svc.insert(ctx, 1, 7, Notification{Title: "Mine", Body: "keep"})
	require.NoError(t, err)
	theirs, err := svc.insert(ctx, 1, 8, Notification{Title: "Theirs", Body: "do not touch"})
	require.NoError(t, err)

	err = svc.MarkRead(ctx, 1, 7, theirs.ID)
	require.ErrorIs(t, err, ErrNotFound)

	stillTheirs, err := svc.List(ctx, 1, 8)
	require.NoError(t, err)
	require.Equal(t, "Theirs", stillTheirs[0].Title)
	require.False(t, stillTheirs[0].Read)

	require.NoError(t, svc.MarkRead(ctx, 1, 7, mine.ID))

	mineList, err := svc.List(ctx, 1, 7)
	require.NoError(t, err)
	require.True(t, mineList[0].Read)
	require.Equal(t, "Mine", mineList[0].Title)

	count, err := svc.UnreadCount(ctx, 1, 7)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	otherCount, err := svc.UnreadCount(ctx, 1, 8)
	require.NoError(t, err)
	require.Equal(t, 1, otherCount)
}

func TestMarkAllReadClearsOnlyTheCallersUnread(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.insert(ctx, 1, 7, Notification{Title: "One", Body: "a"})
	require.NoError(t, err)
	_, err = svc.insert(ctx, 1, 7, Notification{Title: "Two", Body: "b"})
	require.NoError(t, err)
	_, err = svc.insert(ctx, 2, 7, Notification{Title: "Other org", Body: "c"})
	require.NoError(t, err)

	require.NoError(t, svc.MarkAllRead(ctx, 1, 7))

	count, err := svc.UnreadCount(ctx, 1, 7)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	otherOrg, err := svc.UnreadCount(ctx, 2, 7)
	require.NoError(t, err)
	require.Equal(t, 1, otherOrg)

	listed, err := svc.List(ctx, 1, 7)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.True(t, listed[0].Read)
	require.True(t, listed[1].Read)
}
