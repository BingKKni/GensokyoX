package recentmsg

import "sync"

const maxMessagesPerGroup = 200

type GroupMessage struct {
	EventType     string `json:"event_type"`
	MessageID     int    `json:"message_id"`
	RealMessageID string `json:"real_message_id"`
	GroupID       string `json:"group_id"`
	RealGroupID   string `json:"real_group_id"`
	UserID        string `json:"user_id"`
	RealUserID    string `json:"real_user_id"`
	Content       string `json:"content"`
	ReceivedAt    int64  `json:"received_at"`
	RawEvent      string `json:"raw_event,omitempty"`
}

var store = struct {
	sync.RWMutex
	groups map[string][]GroupMessage
}{
	groups: make(map[string][]GroupMessage),
}

func AddGroupMessage(message GroupMessage) {
	if message.GroupID == "" {
		return
	}

	store.Lock()
	defer store.Unlock()

	messages := append(store.groups[message.GroupID], message)
	if len(messages) > maxMessagesPerGroup {
		messages = messages[len(messages)-maxMessagesPerGroup:]
	}
	store.groups[message.GroupID] = messages
}

func GetGroupMessages(groupID string, limit int) []GroupMessage {
	if limit <= 0 {
		return []GroupMessage{}
	}
	if limit > maxMessagesPerGroup {
		limit = maxMessagesPerGroup
	}

	store.RLock()
	defer store.RUnlock()

	messages := store.groups[groupID]
	if len(messages) == 0 {
		return []GroupMessage{}
	}

	if limit > len(messages) {
		limit = len(messages)
	}

	result := make([]GroupMessage, 0, limit)
	for i := len(messages) - 1; i >= 0 && len(result) < limit; i-- {
		result = append(result, messages[i])
	}
	return result
}
