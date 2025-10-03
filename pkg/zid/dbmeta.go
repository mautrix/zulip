package zid

type UserLoginMetadata struct {
	URL   string `json:"url"`
	Email string `json:"email"`
	Token string `json:"token"`

	QueueID     string `json:"queue_id,omitempty"`
	LastEventID int    `json:"last_event_id,omitempty"`
}
