package telegram_test

import (
	"testing"

	"github.com/enricojoe/dailychecker/internal/telegram"
)

// TestParseStartToken verifies that /start <token> messages are parsed
// correctly and non-start messages are silently ignored.
// These tests are pure unit tests — no network or database involved.
func TestParseStartToken_ExtractsToken(t *testing.T) {
	const chatID = int64(99)

	cases := []struct {
		name string
		text string
		want string
	}{
		{
			name: "plain start with token",
			text: "/start abc123",
			want: "abc123",
		},
		{
			name: "start with hex token (real format)",
			text: "/start a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
			want: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
		},
		{
			name: "leading/trailing spaces trimmed — still extracts token",
			text: "  /start   mytoken  ",
			want: "mytoken",
		},
		{
			name: "bare start (no token)",
			text: "/start",
			want: "",
		},
		{
			name: "non-start command",
			text: "/help",
			want: "",
		},
		{
			name: "plain text",
			text: "hello world",
			want: "",
		},
		{
			name: "empty text",
			text: "",
			want: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			u := telegram.Update{
				UpdateID: 1,
				Message: &telegram.Message{
					Text: tc.text,
					Chat: telegram.Chat{ID: chatID},
				},
			}
			got := telegram.ParseStartToken(u)
			if got != tc.want {
				t.Errorf("ParseStartToken(%q): want %q, got %q", tc.text, tc.want, got)
			}
		})
	}
}

func TestParseStartToken_NilMessage(t *testing.T) {
	u := telegram.Update{UpdateID: 1, Message: nil}
	got := telegram.ParseStartToken(u)
	if got != "" {
		t.Errorf("nil message: want empty, got %q", got)
	}
}
