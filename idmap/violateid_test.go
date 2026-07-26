package idmap

import "testing"

func TestIsViolateID(t *testing.T) {
	tests := []struct {
		id      int64
		violate bool
	}{
		{id: 8964, violate: true},
		{id: 89640, violate: true},
		{id: 896400, violate: true},
		{id: 189641, violate: true},
		{id: 16464892, violate: false},
		{id: 890604, violate: true},
		{id: 19890604, violate: true},
		{id: 6464, violate: false},
		{id: 6489, violate: false},
		{id: 6984, violate: false},
		{id: 8963, violate: false},
		{id: 8965, violate: false},
		{id: 10963782, violate: false},
		{id: 0, violate: false},
	}
	for _, test := range tests {
		if got := IsViolateID(test.id); got != test.violate {
			t.Errorf("IsViolateID(%d) = %v; want %v", test.id, got, test.violate)
		}
	}
}

func BenchmarkIsViolateID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = IsViolateID(10963782)
	}
}
