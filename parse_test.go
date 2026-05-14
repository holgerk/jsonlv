package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseLineTime(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantNil bool
		check   func(t *testing.T, got time.Time)
	}{
		{
			name: "datetime key RFC3339Nano with offset (Laravel/Monolog)",
			line: `{"datetime":"2024-01-15T11:07:47.639394+01:00"}`,
			check: func(t *testing.T, got time.Time) {
				assert.False(t, got.IsZero())
				assert.Equal(t, 2024, got.Year())
				assert.Equal(t, time.January, got.Month())
				assert.Equal(t, 15, got.Day())
			},
		},
		{
			name: "timestamp key RFC3339 with Z",
			line: `{"timestamp":"2024-01-15T11:07:47Z"}`,
			check: func(t *testing.T, got time.Time) {
				assert.False(t, got.IsZero())
				assert.Equal(t, int64(1705316867), got.Unix())
			},
		},
		{
			name: "datetime key space-separated (Monolog v2)",
			line: `{"datetime":"2024-01-15 11:07:47"}`,
			check: func(t *testing.T, got time.Time) {
				assert.False(t, got.IsZero())
				assert.Equal(t, 11, got.Hour())
				assert.Equal(t, 7, got.Minute())
				assert.Equal(t, 47, got.Second())
			},
		},
		{
			name:  "datetime key space-separated with fractional seconds",
			line:  `{"datetime":"2024-01-15 11:07:47.639"}`,
			check: func(t *testing.T, got time.Time) { assert.False(t, got.IsZero()) },
		},
		{
			name: "time key unix milliseconds (pino)",
			line: `{"time":1704067200268}`,
			check: func(t *testing.T, got time.Time) {
				assert.False(t, got.IsZero())
				assert.Equal(t, int64(1704067200268), got.UnixMilli())
			},
		},
		{
			name: "timestamp key unix seconds",
			line: `{"timestamp":1704067200}`,
			check: func(t *testing.T, got time.Time) {
				assert.False(t, got.IsZero())
				assert.Equal(t, int64(1704067200), got.Unix())
			},
		},
		{
			name:  "@timestamp key",
			line:  `{"@timestamp":"2024-01-15T11:07:47Z"}`,
			check: func(t *testing.T, got time.Time) { assert.False(t, got.IsZero()) },
		},
		{
			name:  "time key as string",
			line:  `{"time":"2024-01-15T11:07:47.000Z"}`,
			check: func(t *testing.T, got time.Time) { assert.False(t, got.IsZero()) },
		},
		{
			name:  "no timestamp key",
			line:  `{"message":"hello","level":"INFO"}`,
			check: func(t *testing.T, got time.Time) { assert.True(t, got.IsZero()) },
		},
		{
			name:  "unparseable timestamp string",
			line:  `{"datetime":"not-a-date"}`,
			check: func(t *testing.T, got time.Time) { assert.True(t, got.IsZero()) },
		},
		{
			name:  "not JSON",
			line:  "plain text log line",
			check: func(t *testing.T, got time.Time) { assert.True(t, got.IsZero()) },
		},
		{
			name:  "empty string",
			line:  "",
			check: func(t *testing.T, got time.Time) { assert.True(t, got.IsZero()) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, parseLineTime(tt.line))
		})
	}
}
