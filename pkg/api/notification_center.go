package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/open-feature/go-sdk/openfeature"

	"github.com/grafana/grafana/pkg/api/response"
	contextmodel "github.com/grafana/grafana/pkg/services/contexthandler/model"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/notificationcenter"
	"github.com/grafana/grafana/pkg/web"
)

type markNotificationsReadCommand struct {
	ID  string `json:"id"`
	All bool   `json:"all"`
}

func notificationCenterEnabled(ctx context.Context) bool {
	return openfeature.NewDefaultClient().Boolean(
		ctx,
		featuremgmt.FlagGrafanaNotificationCenter,
		false,
		openfeature.TransactionContext(ctx),
	)
}

func (hs *HTTPServer) rejectNotificationCenterRequest(c *contextmodel.ReqContext) response.Response {
	if !notificationCenterEnabled(c.Req.Context()) {
		return response.Error(http.StatusNotFound, "Not found", nil)
	}
	if c.SignedInUser == nil || !c.IsSignedIn || c.IsAnonymous || c.UserID <= 0 {
		return response.Error(http.StatusUnauthorized, "Unauthorized", nil)
	}
	return nil
}

// GET /api/user/notifications
func (hs *HTTPServer) GetUserNotifications(c *contextmodel.ReqContext) response.Response {
	if rejected := hs.rejectNotificationCenterRequest(c); rejected != nil {
		return rejected
	}
	items, err := hs.notificationCenter.List(c.Req.Context(), c.OrgID, c.UserID)
	if err != nil {
		return response.Error(http.StatusInternalServerError, "Failed to load notifications", err)
	}
	return response.JSON(http.StatusOK, map[string]any{"notifications": items})
}

// GET /api/user/notifications/unread-count
func (hs *HTTPServer) GetUserNotificationUnreadCount(c *contextmodel.ReqContext) response.Response {
	if rejected := hs.rejectNotificationCenterRequest(c); rejected != nil {
		return rejected
	}
	count, err := hs.notificationCenter.UnreadCount(c.Req.Context(), c.OrgID, c.UserID)
	if err != nil {
		return response.Error(http.StatusInternalServerError, "Failed to load notifications", err)
	}
	return response.JSON(http.StatusOK, map[string]int{"count": count})
}

// POST /api/user/notifications/read
// Body is {"id":"<notification id>"} or {"all":true}. The user id always comes from the session.
func (hs *HTTPServer) MarkUserNotificationsRead(c *contextmodel.ReqContext) response.Response {
	if rejected := hs.rejectNotificationCenterRequest(c); rejected != nil {
		return rejected
	}
	cmd := markNotificationsReadCommand{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	if !cmd.All && cmd.ID == "" {
		return response.Error(http.StatusBadRequest, "Notification id is required", nil)
	}

	var err error
	if cmd.All {
		err = hs.notificationCenter.MarkAllRead(c.Req.Context(), c.OrgID, c.UserID)
	} else {
		err = hs.notificationCenter.MarkRead(c.Req.Context(), c.OrgID, c.UserID, cmd.ID)
	}
	if errors.Is(err, notificationcenter.ErrNotFound) {
		return response.Error(http.StatusNotFound, "Notification not found", err)
	}
	if err != nil {
		return response.Error(http.StatusInternalServerError, "Failed to mark notifications read", err)
	}

	count, err := hs.notificationCenter.UnreadCount(c.Req.Context(), c.OrgID, c.UserID)
	if err != nil {
		return response.Error(http.StatusInternalServerError, "Failed to load notifications", err)
	}
	return response.JSON(http.StatusOK, map[string]int{"count": count})
}
