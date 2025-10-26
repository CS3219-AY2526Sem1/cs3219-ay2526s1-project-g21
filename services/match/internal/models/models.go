package models

import (
	"time"
)

// Difficulty levels
const (
	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"
)

type JoinReq struct {
	UserID     string `json:"userId"`
	Category   string `json:"category"`
	Difficulty string `json:"difficulty"`
}

type Resp struct {
	OK   bool        `json:"ok"`
	Info interface{} `json:"info"`
}

type RoomInfo struct {
	MatchId    string `json:"matchId"`
	User1      string `json:"user1"`
	User2      string `json:"user2"`
	Category   string `json:"category"`
	Difficulty string `json:"difficulty"`
	Status     string `json:"status"`
	Token1     string `json:"token1"`
	Token2     string `json:"token2"`
	CreatedAt  string `json:"createdAt"`
}

type PendingMatch struct {
	MatchId    string
	User1      string
	User2      string
	Category   string
	Difficulty string
	User1Cat   string
	User1Diff  string
	User2Cat   string
	User2Diff  string
	Token1     string
	Token2     string
	Handshakes map[string]bool
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

type HandshakeReq struct {
	UserID  string `json:"userId"`
	MatchId string `json:"matchId"`
	Accept  bool   `json:"accept"`
}

type CheckResp struct {
	InRoom     bool   `json:"inRoom"`
	RoomId     string `json:"roomId,omitempty"`
	Category   string `json:"category,omitempty"`
	Difficulty string `json:"difficulty,omitempty"`
	Token      string `json:"token,omitempty"`
}
