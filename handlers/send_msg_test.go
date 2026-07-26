package handlers

import (
	"strconv"
	"testing"

	"github.com/hoshinonyaruko/gensokyo/echo"
)

func TestResolveExplicitEventIDFromMemory(t *testing.T) {
	rawEventID := "GROUP_MEMBER_ADD:11111111-2222-3333-4444-555555555555"
	row, err := echo.StoreCacheInMemory(rawEventID)
	if err != nil {
		t.Fatalf("cache event ID: %v", err)
	}

	got := resolveExplicitEventID(strconv.FormatInt(row, 10))
	want := "11111111-2222-3333-4444-555555555555"
	if got != want {
		t.Fatalf("resolved event ID = %q; want %q", got, want)
	}
}

func TestNormalizePlatformEventID(t *testing.T) {
	if got := normalizePlatformEventID("GROUP_MEMBER_REMOVE:event-id"); got != "event-id" {
		t.Fatalf("normalized event ID = %q", got)
	}
	if got := normalizePlatformEventID("event-id"); got != "event-id" {
		t.Fatalf("plain event ID changed to %q", got)
	}
}
