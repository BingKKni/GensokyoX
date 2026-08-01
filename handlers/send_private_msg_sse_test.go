package handlers

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hoshinonyaruko/gensokyo/structs"
)

func TestGenerateMessageSSESequenceAndReset(t *testing.T) {
	const messageID = "test-message-id"
	clearStreamState(messageID)
	t.Cleanup(func() { clearStreamState(messageID) })

	first := generateMessageSSE(structs.InterfaceBody{Content: "first", State: 1}, messageID, "")
	if first.Stream.Index != 0 || first.Stream.ID != "" || first.Stream.Reset {
		t.Fatalf("unexpected first fragment: %+v", first.Stream)
	}

	UpdateRelatedID(messageID, "stream-id")
	second := generateMessageSSE(structs.InterfaceBody{Content: "second", State: 1}, messageID, GetRelatedID(messageID))
	if second.Stream.Index != 1 || second.Stream.ID != "stream-id" || second.Stream.Reset {
		t.Fatalf("unexpected second fragment: %+v", second.Stream)
	}

	reset := generateMessageSSE(structs.InterfaceBody{Content: "replacement", State: 10, Reset: true}, messageID, GetRelatedID(messageID))
	if reset.Stream.Index != 2 || reset.Stream.ID != "stream-id" || !reset.Stream.Reset {
		t.Fatalf("unexpected reset fragment: %+v", reset.Stream)
	}

	payload, err := json.Marshal(reset)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(payload)
	if !strings.Contains(serialized, `"index":2`) || !strings.Contains(serialized, `"reset":true`) {
		t.Fatalf("reset fields missing from JSON: %s", serialized)
	}
}

func TestIncrementIndexConcurrent(t *testing.T) {
	const messageID = "concurrent-message-id"
	const workers = 100
	clearStreamState(messageID)
	t.Cleanup(func() { clearStreamState(messageID) })

	indexes := make([]int, workers)
	var wait sync.WaitGroup
	for i := range indexes {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			indexes[i] = incrementIndex(messageID)
		}(i)
	}
	wait.Wait()
	sort.Ints(indexes)
	for i, index := range indexes {
		if index != i {
			t.Fatalf("index %d was %d", i, index)
		}
	}
}

func TestRollbackIndexAllowsRetry(t *testing.T) {
	const messageID = "retry-message-id"
	clearStreamState(messageID)
	t.Cleanup(func() { clearStreamState(messageID) })

	if index := incrementIndex(messageID); index != 0 {
		t.Fatalf("first index = %d", index)
	}
	rollbackIndex(messageID, 0)
	if index := incrementIndex(messageID); index != 0 {
		t.Fatalf("retried first index = %d", index)
	}
	if index := incrementIndex(messageID); index != 1 {
		t.Fatalf("second index = %d", index)
	}
	rollbackIndex(messageID, 1)
	if index := incrementIndex(messageID); index != 1 {
		t.Fatalf("retried second index = %d", index)
	}
}

func TestClearStreamState(t *testing.T) {
	const messageID = "completed-message-id"
	generateMessageSSE(structs.InterfaceBody{State: 1}, messageID, "")
	UpdateRelatedID(messageID, "stream-id")

	clearStreamState(messageID)

	if relatedID := GetRelatedID(messageID); relatedID != "" {
		t.Fatalf("related stream ID was not cleared: %q", relatedID)
	}
	first := generateMessageSSE(structs.InterfaceBody{State: 1}, messageID, "")
	if first.Stream.Index != 0 {
		t.Fatalf("completed stream index was not cleared: %d", first.Stream.Index)
	}
	clearStreamState(messageID)
}
