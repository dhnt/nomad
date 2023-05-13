package server

import (
	"reflect"
	"strings"
	"testing"
)

func TestResolveArgs(t *testing.T) {
	const root = "/root"

	tests := []struct {
		args     string
		options  []string
		expected string
	}{
		{"", nil, ""},
		{"file:", nil, "/root"},
		{"/", nil, "/"},
		{"file:/", nil, "/root"},
		{".", nil, "."},
		{"file:.", nil, "/root"},
		{"file:..", nil, "/root"},
		{"-al ./", nil, "-al ./"},
		{"-al ./", []string{"-al"}, "-al /root"},
		{"-al file:/./", nil, "-al /root"},
		{"-al ./tmp", nil, "-al ./tmp"},
		{"-al ./tmp", []string{"-al"}, "-al /root/tmp"},
		{"-al file:/./tmp", nil, "-al /root/tmp"},
		{"-al file:/tmp", nil, "-al /root/tmp"},
		{"-al file:///tmp", nil, "-al /root/tmp"},
		{"-al /tmp /bin /etc", []string{"-al"}, "-al /root/tmp /bin /etc"},
		{"-al /tmp file:/bin /etc", []string{"-al"}, "-al /root/tmp /root/bin /etc"},
		{"-al file:/tmp file:/bin file:/etc", []string{"-al"}, "-al /root/tmp /root/bin /root/etc"},
		{"xf /tmp/tar.gz --zstd --strip 1 -C /opt/", nil, "xf /tmp/tar.gz --zstd --strip 1 -C /opt/"},
		{"-xf /tmp/tar.gz --zstd --strip 1 -C /opt/", []string{"-xf"}, "-xf /root/tmp/tar.gz --zstd --strip 1 -C /opt/"},
		{"-xf /tmp/tar.gz --zstd --strip 1 -C /opt/", []string{"-xf", "-C"}, "-xf /root/tmp/tar.gz --zstd --strip 1 -C /root/opt"},
		{"xf file:/tmp/tar.gz --zstd --strip 1 -C file:/opt/", nil, "xf /root/tmp/tar.gz --zstd --strip 1 -C /root/opt"},
	}

	for i, tc := range tests {
		args := strings.Split(tc.args, " ")
		expected := strings.Split(tc.expected, " ")
		resolved, err := resolveArgs(root, tc.options, args)
		if err != nil {
			t.Fatalf("[%v] err: %v", i, err)
		}
		if !reflect.DeepEqual(resolved, expected) {
			t.Fatalf("[%v] args: %v, want: %v got: %v", i, args, tc.expected, resolved)
		}
	}
}
