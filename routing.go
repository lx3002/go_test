package main

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var safeNamePattern = regexp.MustCompile(`[^a-z0-9_-]+`)

func normalizeRoomName(value string) string {
	room := strings.ToLower(strings.TrimSpace(value))
	room = safeNamePattern.ReplaceAllString(room, "-")
	room = strings.Trim(room, "-_")
	if room == "" {
		return "general"
	}
	if len(room) > 48 {
		return room[:48]
	}
	return room
}

func normalizeUserKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func privateFlag(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "private":
		return true
	default:
		return false
	}
}

func roomChannelKey(room string, private bool, secret string) string {
	room = normalizeRoomName(room)
	if !private {
		return "room:" + room
	}

	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return "private:" + room + ":" + hex.EncodeToString(sum[:])[:24]
}

func roomMessageType(private bool) string {
	if private {
		return "private_room"
	}
	return "room"
}
