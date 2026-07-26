package echo

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/idmap"
)

type messageRecord struct {
	messageID  string
	timestamp  time.Time
	usageCount int // 新增字段，记录使用次数，默认为0
}

type messageStore struct {
	mu      sync.Mutex
	records map[string][]messageRecord
}

var instance *messageStore
var once sync.Once

// 惰性初始化
func initInstance() *messageStore {
	once.Do(func() {
		instance = &messageStore{records: make(map[string][]messageRecord)}
	})
	return instance
}

// AddLazyMessageId 添加 message_id 和它的时间戳到指定群号
func AddLazyMessageId(groupID, messageID string, timestamp time.Time) {
	addLazyMessage(groupID, messageID, timestamp)
}

// AddLazyMessageIdv2 添加 message_id 和它的时间戳到指定群号
func AddLazyMessageIdv2(groupID, userID, messageID string, timestamp time.Time) {
	addLazyMessage(groupID+"."+userID, messageID, timestamp)
}

func addLazyMessage(key, messageID string, timestamp time.Time) {
	if key == "" || messageID == "" {
		return
	}
	store := initInstance()
	store.mu.Lock()
	defer store.mu.Unlock()

	cutoff := time.Now().Add(-4 * time.Minute)
	records := store.records[key]
	valid := records[:0]
	for _, record := range records {
		if record.timestamp.After(cutoff) {
			valid = append(valid, record)
		}
	}
	store.records[key] = append(valid, messageRecord{messageID: messageID, timestamp: timestamp})
}

// GetLazyMessagesId 获取指定群号中最近 4 分钟内的 message_id
func GetLazyMessagesId(groupID string) string {
	if messageID := getLazyMessage(groupID); messageID != "" {
		return messageID
	}
	return generateDefaultMessageID(groupID)
}

func GetLazyMessagesIdv2(groupID, userID string) string {
	if messageID := getLazyMessage(groupID + "." + userID); messageID != "" {
		return messageID
	}
	return generateDefaultMessageID(groupID)
}

func getLazyMessage(key string) string {
	store := initInstance()
	store.mu.Lock()
	defer store.mu.Unlock()

	records := store.records[key]
	if len(records) == 0 {
		return ""
	}

	cutoff := time.Now().Add(-4 * time.Minute)
	valid := records[:0]
	for _, record := range records {
		if record.timestamp.After(cutoff) {
			valid = append(valid, record)
		}
	}
	if len(valid) == 0 {
		delete(store.records, key)
		return ""
	}

	selected := -1
	for i := range valid {
		if valid[i].usageCount == 0 && (selected == -1 || valid[i].timestamp.After(valid[selected].timestamp)) {
			selected = i
		}
	}
	if selected == -1 {
		for i := range valid {
			if selected == -1 || valid[i].timestamp.After(valid[selected].timestamp) {
				selected = i
			}
		}
	}

	valid[selected].usageCount++
	store.records[key] = valid
	return valid[selected].messageID
}

// 生成默认消息ID的逻辑拆分为独立函数
func generateDefaultMessageID(groupID string) string {
	virtualGroupID := groupID
	if _, err := strconv.ParseInt(groupID, 10, 64); err == nil {
		if _, err := idmap.RetrieveRowByIDv2(groupID); err != nil {
			if _, resolved, resolveErr := idmap.RetrieveVirtualValuev2(groupID); resolveErr == nil {
				virtualGroupID = resolved
			}
		}
	} else if _, resolved, err := idmap.RetrieveVirtualValuev2(groupID); err == nil {
		virtualGroupID = resolved
	}
	msgType := GetMessageTypeByGroupidv2(config.GetAppIDStr(), virtualGroupID)
	if strings.HasPrefix(msgType, "guild") {
		return "1000"
	}
	return "2000"
}

// 通过group_id获取类型
func GetMessageTypeByGroupidv2(appID string, GroupID interface{}) string { //2
	// 从appID和userID生成key
	var GroupIDStr string
	switch u := GroupID.(type) {
	case int:
		GroupIDStr = strconv.Itoa(u)
	case int64:
		GroupIDStr = strconv.FormatInt(u, 10)
	case string:
		GroupIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}

	key := appID + "_" + GroupIDStr
	return GetMsgTypeByKey(key)
}
