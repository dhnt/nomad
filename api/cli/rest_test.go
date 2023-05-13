package cli

import (
	"syscall"
	"testing"
)

func TestStatfs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test...")
	}

	cli, err := NewClient("http://localhost:58080/")
	if err != nil {
		t.FailNow()
	}

	var st syscall.Statfs_t
	err = cli.Statfs("/test", &st)
	if err != nil {
		t.Fatalf("%v", err)
	}

	t.Logf("%v", st)
}

func TestStat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test...")
	}

	cli, err := NewClient("http://localhost:58080/")
	if err != nil {
		t.FailNow()
	}

	var st syscall.Stat_t
	err = cli.Stat("/test", &st)
	if err != nil {
		t.Fatalf("%v", err)
	}

	t.Logf("%v", st)
}

func TestIsNilInterface(t *testing.T) {
	var a []int
	var b *bool
	f := func(x interface{}) interface{} {
		return x
	}
	var i interface{}
	var m map[string]interface{}
	var s *string
	var ts *struct{}

	tests := []struct {
		i        interface{}
		expected bool
	}{
		{nil, true},
		{a, true},
		{b, true},
		{f(nil), true},
		{i, true},
		{m, true},
		{s, true},
		{ts, true},
		{[]int{}, false},
		{true, false},
		{f(true), false},
		{[]interface{}{}, false},
		{"", false},
	}

	for _, tc := range tests {
		v := isNilInterface(tc.i)
		if tc.expected != v {
			t.FailNow()
		}
	}
}
