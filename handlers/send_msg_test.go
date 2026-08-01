package handlers

import (
	"strconv"
	"testing"

	"github.com/hoshinonyaruko/gensokyo/echo"
)

func TestResolveExplicitEventIDFromMemory(t *testing.T) {
	// 平台被动回复要求 event_id 保持上报时的完整形态(含事件类型前缀),
	// 裁剪成裸 uuid 会被平台判 40034025, 这里断言前缀被完整保留
	rawEventID := "GROUP_MEMBER_ADD:11111111-2222-3333-4444-555555555555"
	row, err := echo.StoreCacheInMemory(rawEventID)
	if err != nil {
		t.Fatalf("cache event ID: %v", err)
	}

	got := resolveExplicitEventID(strconv.FormatInt(row, 10))
	if got != rawEventID {
		t.Fatalf("resolved event ID = %q; want %q", got, rawEventID)
	}
}

func TestResolveExplicitEventIDPassthrough(t *testing.T) {
	// 非虚拟ID(平台原生 event_id)应原样透传, 不做任何裁剪
	for _, id := range []string{
		"GROUP_MEMBER_REMOVE:22222222-3333-4444-5555-666666666666",
		"INTERACTION_CREATE:33333333-4444-5555-6666-777777777777",
	} {
		if got := resolveExplicitEventID(id); got != id {
			t.Fatalf("event ID %q changed to %q", id, got)
		}
	}
}
