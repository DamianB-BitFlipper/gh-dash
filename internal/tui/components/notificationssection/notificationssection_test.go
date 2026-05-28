package notificationssection

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/notificationrow"
)

func TestLocalSearchFiltersNotificationsByTitleRepoAndReason(t *testing.T) {
	first := notificationrow.Data{
		Notification: data.NotificationData{
			Id:     "1",
			Reason: "review_requested",
			Subject: data.NotificationSubject{
				Title: "Review math PR",
				Type:  "PullRequest",
				Url:   "https://api.github.com/repos/owner/calculator/pulls/123",
			},
			Repository: data.NotificationRepository{FullName: "owner/calculator"},
		},
	}
	second := notificationrow.Data{
		Notification: data.NotificationData{
			Id:     "2",
			Reason: "mention",
			Subject: data.NotificationSubject{
				Title: "Docs typo",
				Type:  "Issue",
				Url:   "https://api.github.com/repos/owner/docs/issues/456",
			},
			Repository: data.NotificationRepository{FullName: "owner/docs"},
		},
	}
	m := Model{Notifications: []notificationrow.Data{first, second}}

	m.LocalSearchValue = "math"
	require.Len(t, m.filteredNotifications(), 1)
	require.Equal(t, "1", m.GetCurrNotification().GetId())

	m.LocalSearchValue = "#456"
	require.Len(t, m.filteredNotifications(), 1)
	require.Equal(t, "2", m.filteredNotifications()[0].GetId())

	m.LocalSearchValue = "review_requested"
	require.Len(t, m.filteredNotifications(), 1)
	require.Equal(t, "1", m.filteredNotifications()[0].GetId())
}
