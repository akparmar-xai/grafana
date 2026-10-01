package notificationcenter

import "time"

// Notification is one in-app message for a single user.
// Title and Body are plain text. Callers must render them as text, never as HTML.
type Notification struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
	Read    bool      `json:"read"`
}
