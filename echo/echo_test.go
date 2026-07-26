package echo

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hoshinonyaruko/gensokyo/idmap"
)

func resetMemoryMessageIDs() {
	globalMemoryMessageIDs.mu.Lock()
	defer globalMemoryMessageIDs.mu.Unlock()
	globalMemoryMessageIDs.byID = make(map[string]int64)
	globalMemoryMessageIDs.byRow = make(map[int64]memoryMessageIDEntry)
}

func TestMemoryMessageIDRoundTripAndIdempotence(t *testing.T) {
	resetMemoryMessageIDs()
	t.Cleanup(resetMemoryMessageIDs)

	first, err := StoreCacheInMemory("platform-message-id")
	if err != nil {
		t.Fatalf("store message ID: %v", err)
	}
	second, err := StoreCacheInMemory("platform-message-id")
	if err != nil || second != first {
		t.Fatalf("second row = %d, %v; want %d", second, err, first)
	}
	got, ok := GetCacheIDFromMemoryByRowID(strconv.FormatInt(first, 10))
	if !ok || got != "platform-message-id" {
		t.Fatalf("round trip = %q, %v", got, ok)
	}
}

func TestMemoryMessageIDCollisionDoesNotOverwrite(t *testing.T) {
	resetMemoryMessageIDs()
	t.Cleanup(resetMemoryMessageIDs)

	id := "colliding-platform-message"
	collidingRow, err := idmap.GenerateRowID(id, 9)
	if err != nil {
		t.Fatalf("generate row: %v", err)
	}
	globalMemoryMessageIDs.byID["existing-message"] = collidingRow
	globalMemoryMessageIDs.byRow[collidingRow] = memoryMessageIDEntry{
		id:        "existing-message",
		expiresAt: time.Now().Add(time.Hour),
	}

	row, err := StoreCacheInMemory(id)
	if err != nil {
		t.Fatalf("store colliding message: %v", err)
	}
	if row == collidingRow {
		t.Fatalf("collision overwrote row %d", row)
	}
	got, ok := GetCacheIDFromMemoryByRowID(strconv.FormatInt(collidingRow, 10))
	if !ok || got != "existing-message" {
		t.Fatalf("existing reverse mapping changed to %q, %v", got, ok)
	}
}

func TestMemoryMessageIDExpiryRemovesBothDirections(t *testing.T) {
	resetMemoryMessageIDs()
	t.Cleanup(resetMemoryMessageIDs)

	row, err := StoreCacheInMemory("expired-message")
	if err != nil {
		t.Fatalf("store message ID: %v", err)
	}
	entry := globalMemoryMessageIDs.byRow[row]
	entry.expiresAt = time.Now().Add(-time.Second)
	globalMemoryMessageIDs.byRow[row] = entry

	if _, ok := GetCacheIDFromMemoryByRowID(strconv.FormatInt(row, 10)); ok {
		t.Fatal("expired reverse mapping was returned")
	}
	if _, ok := globalMemoryMessageIDs.byID["expired-message"]; ok {
		t.Fatal("expired forward mapping was not removed")
	}
}

func TestEchoCleanupKeepsFreshEntries(t *testing.T) {
	mapping := &EchoMapping{}
	storeExpiringString(&mapping.msgIDMapping, "fresh", "message")
	mapping.msgIDMapping.Store("expired", expiringString{
		value:     "old-message",
		expiresAt: time.Now().Add(-time.Second),
	})

	cleanupEchoMapping(mapping)
	if got := loadExpiringString(&mapping.msgIDMapping, "fresh"); got != "message" {
		t.Fatalf("fresh entry = %q; want message", got)
	}
	if got := loadExpiringString(&mapping.msgIDMapping, "expired"); got != "" {
		t.Fatalf("expired entry = %q; want empty", got)
	}
}

func resetLazyMessages() {
	store := initInstance()
	store.mu.Lock()
	defer store.mu.Unlock()
	store.records = make(map[string][]messageRecord)
}

func TestLazyMessagesPreferNewestUnusedRecord(t *testing.T) {
	resetLazyMessages()
	t.Cleanup(resetLazyMessages)

	now := time.Now()
	AddLazyMessageId("group", "older", now.Add(-time.Minute))
	AddLazyMessageId("group", "newer", now)

	if got := getLazyMessage("group"); got != "newer" {
		t.Fatalf("first selection = %q; want newer", got)
	}
	if got := getLazyMessage("group"); got != "older" {
		t.Fatalf("second selection = %q; want older unused record", got)
	}
	if got := getLazyMessage("group"); got != "newer" {
		t.Fatalf("third selection = %q; want newest record", got)
	}
}

func TestLazyMessageConcurrentAddDoesNotLoseRecords(t *testing.T) {
	resetLazyMessages()
	t.Cleanup(resetLazyMessages)

	const count = 100
	var wait sync.WaitGroup
	wait.Add(count)
	for i := 0; i < count; i++ {
		go func(i int) {
			defer wait.Done()
			AddLazyMessageId("group", strconv.Itoa(i), time.Now())
		}(i)
	}
	wait.Wait()

	store := initInstance()
	store.mu.Lock()
	defer store.mu.Unlock()
	if got := len(store.records["group"]); got != count {
		t.Fatalf("record count = %d; want %d", got, count)
	}
}
