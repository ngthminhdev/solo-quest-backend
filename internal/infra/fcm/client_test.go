package fcm_test

import (
	"errors"
	"testing"

	"solo_quest_backend/internal/infra/fcm"
)

func TestIsInvalidTokenError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"not registered", errors.New("registration-token-not-registered"), true},
		{"unregistered", errors.New("UNREGISTERED"), true},
		{"notregistered", errors.New("NotRegistered"), true},
		{"entity not found", errors.New("requested entity was not found"), true},
		{"invalid token format", errors.New("registration token is not a valid fcm registration token"), true},
		{"unknown error", errors.New("unknown error"), false},
		{"permission denied", errors.New("permission denied"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fcm.IsInvalidTokenError(tt.err)
			if result != tt.expected {
				t.Errorf("IsInvalidTokenError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"internal error", errors.New("internal server error"), true},
		{"unavailable", errors.New("service UNAVAILABLE"), true},
		{"deadline exceeded", errors.New("deadline exceeded"), true},
		{"timeout", errors.New("request TIMEOUT"), true},
		{"permission denied", errors.New("permission denied"), false},
		{"not found", errors.New("not found"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fcm.IsTransientError(tt.err)
			if result != tt.expected {
				t.Errorf("IsTransientError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}
