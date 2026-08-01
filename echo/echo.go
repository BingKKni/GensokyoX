package echo

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/tencent-connect/botgo/dto"
)

const (
	// 私信环境下平台 msg_id 有效期为1小时，统一取60分钟以覆盖该上限。
	echoMappingTTL    = 60 * time.Minute
	messageIDCacheTTL = 60 * time.Minute
	// 群聊被动消息的 event_id 有效期只有5分钟（单聊才是60分钟），
	// 按 echoMappingTTL 缓存会把早已过期的 event_id 兜底发出去，
	// 被平台判 40034026(请求参数event_id已过期)。这里留30秒安全余量。
	eventIDGroupTTL = 4*time.Minute + 30*time.Second
)

type expiringString struct {
	value     string
	expiresAt time.Time
}

type memoryMessageIDEntry struct {
	id        string
	expiresAt time.Time
}

type memoryMessageIDCache struct {
	mu    sync.Mutex
	byID  map[string]int64
	byRow map[int64]memoryMessageIDEntry
}

func init() {
	// 在 init 函数中运行清理逻辑
	startCleanupRoutine()
}

func startCleanupRoutine() {
	ticker := time.NewTicker(30 * time.Minute)
	echoCleanupTicker = ticker
	go func() {
		for range ticker.C {
			cleanupGlobalMaps()
		}
	}()
}

func cleanupGlobalMaps() {
	cleanupMessageGroupStack(globalMessageGroupStack)
	cleanupEchoMapping(globalEchoMapping)
	cleanupInt64ToIntMapping(globalInt64ToIntMapping)
	cleanupStringToIntMappingSeq(globalStringToIntMappingSeq)
}

func cleanupMessageGroupStack(stack *globalMessageGroup) {
	stack.mu.Lock()
	defer stack.mu.Unlock()
	stack.stack = make([]MessageGroupPair, 0)
}

func cleanupEchoMapping(mapping *EchoMapping) {
	now := time.Now()
	cleanupExpiredStrings(&mapping.msgTypeMapping, now)
	cleanupExpiredStrings(&mapping.msgIDMapping, now)
	cleanupExpiredStrings(&mapping.eventIDMapping, now)
}

func cleanupExpiredStrings(mapping *sync.Map, now time.Time) {
	mapping.Range(func(key, value interface{}) bool {
		entry, ok := value.(expiringString)
		if !ok || !entry.expiresAt.After(now) {
			mapping.Delete(key)
		}
		return true
	})
}

func cleanupInt64ToIntMapping(mapping *Int64ToIntMapping) {
	mapping.mapping.Range(func(key, value interface{}) bool {
		mapping.mapping.Delete(key)
		return true
	})
}

func cleanupStringToIntMappingSeq(mapping *StringToIntMappingSeq) {
	mapping.mapping.Range(func(key, value interface{}) bool {
		mapping.mapping.Delete(key)
		return true
	})
}

type EchoMapping struct {
	msgTypeMapping sync.Map
	msgIDMapping   sync.Map
	eventIDMapping sync.Map
}

// Int64ToIntMapping 用于存储 int64 到 int 的映射(递归计数器)
type Int64ToIntMapping struct {
	mapping sync.Map
}

// StringToIntMappingSeq 用于存储 string 到 int 的映射(seq对应)
type StringToIntMappingSeq struct {
	mapping sync.Map
}

// MessageGroupPair 用于存储 group 和 groupMessage
type MessageGroupPair struct {
	Group        string
	GroupMessage *dto.MessageToCreate
}

// 定义全局栈的结构体
type globalMessageGroup struct {
	mu    sync.Mutex
	stack []MessageGroupPair
}

var (
	echoCleanupTicker      *time.Ticker
	messageIDCleanupTicker *time.Ticker
	onceMsgid              sync.Once
)

var globalMemoryMessageIDs = &memoryMessageIDCache{
	byID:  make(map[string]int64),
	byRow: make(map[int64]memoryMessageIDEntry),
}

// 初始化一个全局栈实例
var globalMessageGroupStack = &globalMessageGroup{
	stack: make([]MessageGroupPair, 0),
}

var globalEchoMapping = &EchoMapping{
	msgTypeMapping: sync.Map{},
	msgIDMapping:   sync.Map{},
	eventIDMapping: sync.Map{},
}

var globalInt64ToIntMapping = &Int64ToIntMapping{
	mapping: sync.Map{},
}

var globalStringToIntMappingSeq = &StringToIntMappingSeq{
	mapping: sync.Map{},
}

func (e *EchoMapping) GenerateKey(appid string, s int64) string {
	return appid + "_" + strconv.FormatInt(s, 10)
}

func (e *EchoMapping) GenerateKeyv2(appid string, groupid int64, userid int64) string {
	return appid + "_" + strconv.FormatInt(groupid, 10) + "_" + strconv.FormatInt(userid, 10)
}

func (e *EchoMapping) GenerateKeyEventID(appid string, groupid int64) string {
	return appid + "_" + strconv.FormatInt(groupid, 10)
}

func (e *EchoMapping) GenerateKeyEventIDV2(appid string, groupid string) string {
	return appid + "_" + groupid
}

func (e *EchoMapping) GenerateKeyv3(appid string, s string) string {
	return appid + "_" + s
}

func storeExpiringString(mapping *sync.Map, key, value string) {
	storeExpiringStringWithTTL(mapping, key, value, echoMappingTTL)
}

func storeExpiringStringWithTTL(mapping *sync.Map, key, value string, ttl time.Duration) {
	mapping.Store(key, expiringString{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	})
}

func loadExpiringString(mapping *sync.Map, key string) string {
	value, ok := mapping.Load(key)
	if !ok {
		return ""
	}
	entry, ok := value.(expiringString)
	if !ok || !entry.expiresAt.After(time.Now()) {
		mapping.Delete(key)
		return ""
	}
	return entry.value
}

// 添加 echo 对应的类型
func AddMsgType(appid string, s int64, msgType string) {
	key := globalEchoMapping.GenerateKey(appid, s)
	storeExpiringString(&globalEchoMapping.msgTypeMapping, key, msgType)
}

// 添加echo对应的messageid
func AddMsgIDv3(appid string, s string, msgID string) {
	key := globalEchoMapping.GenerateKeyv3(appid, s)
	storeExpiringString(&globalEchoMapping.msgIDMapping, key, msgID)
}

// GetMsgIDv3 返回给定appid和s的msgID
func GetMsgIDv3(appid string, s string) string {
	key := globalEchoMapping.GenerateKeyv3(appid, s)
	return loadExpiringString(&globalEchoMapping.msgIDMapping, key)
}

// 添加group和userid对应的messageid
func AddMsgIDv2(appid string, groupid int64, userid int64, msgID string) {
	key := globalEchoMapping.GenerateKeyv2(appid, groupid, userid)
	storeExpiringString(&globalEchoMapping.msgIDMapping, key, msgID)
}

// 添加group对应的eventid
func AddEvnetID(appid string, groupid int64, eventID string) {
	key := globalEchoMapping.GenerateKeyEventID(appid, groupid)
	storeExpiringStringWithTTL(&globalEchoMapping.eventIDMapping, key, eventID, eventIDGroupTTL)
}

// 添加group对应的eventid
func AddEvnetIDv2(appid string, groupid string, eventID string) {
	key := globalEchoMapping.GenerateKeyEventIDV2(appid, groupid)
	storeExpiringStringWithTTL(&globalEchoMapping.eventIDMapping, key, eventID, eventIDGroupTTL)
}

// 添加私聊(C2C)用户对应的eventid。私聊被动消息有效期为60分钟, 不适用群聊的5分钟限制,
// key 的生成方式与 AddEvnetID 一致, 取用侧无需区分。
func AddEvnetIDC2C(appid string, userid int64, eventID string) {
	key := globalEchoMapping.GenerateKeyEventID(appid, userid)
	storeExpiringString(&globalEchoMapping.eventIDMapping, key, eventID)
}

// 添加echo对应的messageid
func AddMsgID(appid string, s int64, msgID string) {
	key := globalEchoMapping.GenerateKey(appid, s)
	storeExpiringString(&globalEchoMapping.msgIDMapping, key, msgID)
}

// 根据给定的key获取消息类型
func GetMsgTypeByKey(key string) string {
	return loadExpiringString(&globalEchoMapping.msgTypeMapping, key)
}

// 根据给定的key获取消息ID
func GetMsgIDByKey(key string) string {
	return loadExpiringString(&globalEchoMapping.msgIDMapping, key)
}

// 根据给定的key获取EventID
func GetEventIDByKey(key string) string {
	return loadExpiringString(&globalEchoMapping.eventIDMapping, key)
}

// AddMapping 添加一个新的映射
func AddMapping(key int64, value int) {
	globalInt64ToIntMapping.mapping.Store(key, value)
}

// GetMapping 根据给定的 int64 键获取映射值
func GetMapping(key int64) int {
	value, _ := globalInt64ToIntMapping.mapping.Load(key)
	if value == nil {
		return 0 // 根据需要返回默认值或者进行错误处理
	}
	return value.(int)
}

// AddMappingSeq 添加一个新的映射
func AddMappingSeq(key string, value int) {
	globalStringToIntMappingSeq.mapping.Store(key, value)
}

// GetMappingSeq 根据给定的 string 键获取映射值
func GetMappingSeq(key string) int {
	value, ok := globalStringToIntMappingSeq.mapping.Load(key)
	if !ok {
		if config.GetRamDomSeq() {
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			return rng.Intn(10000) + 1 // 生成 1 到 10000 的随机数
		}
		return 0 // 或者根据需要返回默认值或者进行错误处理
	}
	return value.(int)
}

// PushGlobalStack 向全局栈中添加一个新的 MessageGroupPair
func PushGlobalStack(pair MessageGroupPair) {
	globalMessageGroupStack.mu.Lock()
	defer globalMessageGroupStack.mu.Unlock()
	globalMessageGroupStack.stack = append(globalMessageGroupStack.stack, pair)
}

// PopGlobalStackMulti 从全局栈中取出指定数量的 MessageGroupPair，但不删除它们
func PopGlobalStackMulti(count int) []MessageGroupPair {
	globalMessageGroupStack.mu.Lock()
	defer globalMessageGroupStack.mu.Unlock()

	if count == 0 || len(globalMessageGroupStack.stack) == 0 {
		return nil
	}

	if count > len(globalMessageGroupStack.stack) {
		count = len(globalMessageGroupStack.stack)
	}

	return globalMessageGroupStack.stack[:count]
}

// RemoveFromGlobalStack 从全局栈中删除指定下标的元素
func RemoveFromGlobalStack(index int) {
	globalMessageGroupStack.mu.Lock()
	defer globalMessageGroupStack.mu.Unlock()

	if index < 0 || index >= len(globalMessageGroupStack.stack) {
		return // 下标越界检查
	}

	globalMessageGroupStack.stack = append(globalMessageGroupStack.stack[:index], globalMessageGroupStack.stack[index+1:]...)
}

// StoreCacheInMemory stores a bounded, bidirectional message-ID mapping.
func StoreCacheInMemory(id string) (int64, error) {
	if id == "" {
		return 0, fmt.Errorf("message ID is empty")
	}

	now := time.Now()
	globalMemoryMessageIDs.mu.Lock()
	defer globalMemoryMessageIDs.mu.Unlock()

	if row, ok := globalMemoryMessageIDs.byID[id]; ok {
		if entry, exists := globalMemoryMessageIDs.byRow[row]; exists && entry.id == id && entry.expiresAt.After(now) {
			return row, nil
		}
		delete(globalMemoryMessageIDs.byID, id)
		delete(globalMemoryMessageIDs.byRow, row)
	}

	maxDigits := 18 // int64 的位数上限-1
	for digits := 9; digits <= maxDigits; digits++ {
		newRow, err := idmap.GenerateRowID(id, digits)
		if err != nil {
			return 0, err
		}
		if newRow <= 0 {
			continue
		}
		if existing, exists := globalMemoryMessageIDs.byRow[newRow]; exists {
			if existing.expiresAt.After(now) {
				continue
			}
			delete(globalMemoryMessageIDs.byID, existing.id)
			delete(globalMemoryMessageIDs.byRow, newRow)
		}

		globalMemoryMessageIDs.byID[id] = newRow
		globalMemoryMessageIDs.byRow[newRow] = memoryMessageIDEntry{
			id:        id,
			expiresAt: now.Add(messageIDCacheTTL),
		}
		return newRow, nil
	}
	return 0, fmt.Errorf("unable to find a unique row ID after %d attempts", maxDigits-8)
}

// GetIDFromRowID 根据行号获取原始 ID
func GetCacheIDFromMemoryByRowID(rowID string) (string, bool) {
	introwID, err := strconv.ParseInt(rowID, 10, 64)
	if err != nil {
		return "", false
	}

	now := time.Now()
	globalMemoryMessageIDs.mu.Lock()
	defer globalMemoryMessageIDs.mu.Unlock()
	entry, ok := globalMemoryMessageIDs.byRow[introwID]
	if !ok {
		return "", false
	}
	if !entry.expiresAt.After(now) {
		delete(globalMemoryMessageIDs.byRow, introwID)
		if globalMemoryMessageIDs.byID[entry.id] == introwID {
			delete(globalMemoryMessageIDs.byID, entry.id)
		}
		return "", false
	}
	return entry.id, true
}

func cleanupMemoryMessageIDs(now time.Time) {
	globalMemoryMessageIDs.mu.Lock()
	defer globalMemoryMessageIDs.mu.Unlock()
	for row, entry := range globalMemoryMessageIDs.byRow {
		if entry.expiresAt.After(now) {
			continue
		}
		delete(globalMemoryMessageIDs.byRow, row)
		if globalMemoryMessageIDs.byID[entry.id] == row {
			delete(globalMemoryMessageIDs.byID, entry.id)
		}
	}
}

// StartCleanupRoutine periodically removes expired message-ID mappings.
func StartCleanupRoutine() {
	onceMsgid.Do(func() {
		ticker := time.NewTicker(5 * time.Minute)
		messageIDCleanupTicker = ticker

		go func() {
			for now := range ticker.C {
				cleanupMemoryMessageIDs(now)
			}
		}()
	})
}

// StopCleanupRoutine 停止定时清理函数
func StopCleanupRoutine() {
	if messageIDCleanupTicker != nil {
		messageIDCleanupTicker.Stop()
	}
}
