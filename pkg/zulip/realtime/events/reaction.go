package events

const ReactionType EventType = "reaction"

type Reaction struct {
	ID   int       `json:"id"`
	Op   string    `json:"op"`
	Type EventType `json:"type"`
	ReactionData
	MessageID int `json:"message_id"`
	UserID    int `json:"user_id"`
}

type ReactionData struct {
	EmojiName    string `json:"emoji_name"`
	EmojiCode    string `json:"emoji_code"`
	ReactionType string `json:"reaction_type"`
}

func (e *Reaction) EventID() int {
	return e.ID
}

func (e *Reaction) EventType() EventType {
	return e.Type
}

func (e *Reaction) EventOp() string {
	return e.Op
}
