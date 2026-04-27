package main

import "testing"

func TestParseMemoryLimitBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  int64
	}{
		{name: "mib", value: "3500MiB", want: 3500 << 20},
		{name: "gib decimal", value: "3.5GiB", want: int64(3.5 * float64(1<<30))},
		{name: "mb", value: "512MB", want: 512 * 1000 * 1000},
		{name: "bytes", value: "1024", want: 1024},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseMemoryLimitBytes(tt.value)
			if err != nil {
				t.Fatalf("parseMemoryLimitBytes(%q) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("parseMemoryLimitBytes(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseMemoryLimitBytesRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "0", "-1MiB", "abc"} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if got, err := parseMemoryLimitBytes(value); err == nil {
				t.Fatalf("parseMemoryLimitBytes(%q) = %d, want error", value, got)
			}
		})
	}
}
