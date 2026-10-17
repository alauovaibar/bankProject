package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"notification-service/internal/appErr"
	"notification-service/internal/service"
)

type NotificationHandler struct {
	notificationService service.NotificationService
}

func NewNotificationHandler(notificationService service.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
	}
}

func (h *NotificationHandler) GetUserNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) >= 4 {
			userID = pathParts[len(pathParts)-1]
		}
	}

	if userID == "" {
		appErr.WriteError(w, appErr.NewAppError(errors.New("unauthorized"), "user id is required", http.StatusBadRequest))
		return
	}

	notifications, err := h.notificationService.GetNotifications(r.Context(), userID)
	if err != nil {
		appErr.WriteError(w, appErr.NewAppError(err, "failed to get notifications", http.StatusInternalServerError))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(notifications)
}
