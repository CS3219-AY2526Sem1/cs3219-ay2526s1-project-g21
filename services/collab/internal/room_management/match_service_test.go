package room_management

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"

	"collab/internal/models"
	"collab/internal/utils"
)

func setupRoomManager(t *testing.T, questionHandler http.HandlerFunc) (*RoomManager, *miniredis.Miniredis, *httptest.Server) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	var server *httptest.Server
	if questionHandler != nil {
		server = httptest.NewServer(questionHandler)
		t.Cleanup(server.Close)
	}

	questionURL := ""
	if server != nil {
		questionURL = server.URL
	}

	manager := NewRoomManager(mr.Addr(), questionURL)
	t.Cleanup(func() { _ = manager.rdb.Close() })
	return manager, mr, server
}

func TestSubscribeToMatches(t *testing.T) {
	manager, _, _ := setupRoomManager(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(models.Question{ID: 42, Title: "Q"})
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go manager.SubscribeToMatches(ctx)
	time.Sleep(50 * time.Millisecond)

	event := models.RoomInfo{
		MatchId:    "matchA",
		User1:      "u1",
		User2:      "u2",
		Category:   "algorithms",
		Difficulty: "easy",
		Token1:     "t1",
		Token2:     "t2",
	}
	payload, _ := json.Marshal(event)
	if err := manager.rdb.Publish(context.Background(), "matches", string(payload)).Err(); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		manager.mu.RLock()
		info, ok := manager.roomStatusMap[event.MatchId]
		manager.mu.RUnlock()
		return ok && info.Status == "ready" && info.Question != nil && info.Question.ID == 42
	})
}

func TestSubscribeToMatchesInvalidPayload(t *testing.T) {
	manager, _, _ := setupRoomManager(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(models.Question{ID: 1})
	})
	ctx, cancel := context.WithCancel(context.Background())
	go manager.SubscribeToMatches(ctx)
	time.Sleep(50 * time.Millisecond)

	if err := manager.rdb.Publish(context.Background(), "matches", "not-json").Err(); err != nil {
		t.Fatalf("publish invalid payload: %v", err)
	}

	manager.rdb.Close()
	time.Sleep(100 * time.Millisecond)
	cancel()
}

func TestSubscribeToMatchesWithNilContext(t *testing.T) {
	manager, _, _ := setupRoomManager(t, nil)
	go manager.SubscribeToMatches(context.TODO())
	time.Sleep(50 * time.Millisecond)
	if err := manager.rdb.Publish(context.Background(), "matches", "bad").Err(); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	manager.rdb.Close()
	time.Sleep(100 * time.Millisecond)
}

func TestHandleMatchPayloadInvalid(t *testing.T) {
	manager, _, _ := setupRoomManager(t, nil)
	manager.handleMatchPayload("not-json")
}

func TestHandleMatchPayloadValid(t *testing.T) {
	manager, _, _ := setupRoomManager(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(models.Question{ID: 7})
	})
	manager.handleMatchPayload(`{"matchId":"mh","user1":"a","user2":"b","category":"cat","difficulty":"easy"}`)
	waitUntil(t, 2*time.Second, func() bool {
		manager.mu.RLock()
		_, ok := manager.roomStatusMap["mh"]
		manager.mu.RUnlock()
		return ok
	})
}

func TestProcessMatchEventFetchError(t *testing.T) {
	manager, _, _ := setupRoomManager(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	})
	event := models.RoomInfo{MatchId: "m1", Category: "c", Difficulty: "d"}
	manager.processMatchEvent(event)

	manager.mu.RLock()
	info := manager.roomStatusMap["m1"]
	manager.mu.RUnlock()
	if info == nil || info.Status != "error" || info.Question != nil {
		t.Fatalf("expected error status, got %#v", info)
	}
}

func TestFetchQuestion(t *testing.T) {
	manager, _, server := setupRoomManager(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("difficulty") != "easy" || r.URL.Query().Get("topic") != "graphs" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(models.Question{ID: 7})
	})

	q, err := manager.fetchQuestion("graphs", "easy")
	if err != nil || q.ID != 7 {
		t.Fatalf("unexpected question: %#v err=%v", q, err)
	}

	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	})
	if _, err := manager.fetchQuestion("graphs", "easy"); err == nil {
		t.Fatalf("expected error when service returns non-200")
	}

	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{invalid"))
	})
	if _, err := manager.fetchQuestion("graphs", "easy"); err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestFetchQuestionNetworkError(t *testing.T) {
	manager := NewRoomManager("localhost:0", "http://127.0.0.1:0")
	if _, err := manager.fetchQuestion("cat", "easy"); err == nil {
		t.Fatalf("expected network error")
	}
}

func TestFetchAlternativeQuestion(t *testing.T) {
	var counter atomic.Int64
	manager, _, server := setupRoomManager(t, func(w http.ResponseWriter, r *http.Request) {
		id := counter.Add(1)
		if id <= 2 {
			id = 3 // duplicate id
		}
		_ = json.NewEncoder(w).Encode(models.Question{ID: int(id)})
	})

	q, err := manager.fetchAlternativeQuestion("cat", "hard", 3)
	if err != nil || q.ID == 3 {
		t.Fatalf("expected different question, got %#v err=%v", q, err)
	}

	counter.Store(0)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(models.Question{ID: 5})
	})
	if _, err := manager.fetchAlternativeQuestion("cat", "hard", 5); !errors.Is(err, ErrNoAlternativeQuestion) {
		t.Fatalf("expected ErrNoAlternativeQuestion, got %v", err)
	}

	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusInternalServerError)
	})
	if _, err := manager.fetchAlternativeQuestion("cat", "hard", 0); err == nil {
		t.Fatalf("expected error when fetchQuestion fails")
	}

}

func TestUpdateAndFetchRoomStatusFromRedis(t *testing.T) {
	manager, _, _ := setupRoomManager(t, nil)
	info := &models.RoomInfo{
		MatchId:          "match1",
		User1:            "u1",
		User2:            "u2",
		Category:         "cat",
		Difficulty:       "easy",
		Status:           "ready",
		RerollsRemaining: 2,
		CreatedAt:        "now",
		Token1:           "tok1",
		Token2:           "tok2",
		Question:         &models.Question{ID: 9, Title: "title"},
	}

	manager.updateRoomStatusInRedis(context.Background(), info)
	loaded, err := manager.fetchRoomStatusFromRedis("match1")
	if err != nil {
		t.Fatalf("failed to load from redis: %v", err)
	}
	if loaded.MatchId != info.MatchId || loaded.Question.ID != 9 {
		t.Fatalf("unexpected loaded info: %#v", loaded)
	}

	if _, err := manager.fetchRoomStatusFromRedis("missing"); err == nil {
		t.Fatalf("expected error for missing room")
	}

	infoNil := &models.RoomInfo{MatchId: "match2", Status: "ready"}
	manager.updateRoomStatusInRedis(context.Background(), infoNil)
	stored := manager.rdb.HGetAll(context.Background(), "room:match2").Val()
	if stored["question"] != "" {
		t.Fatalf("expected empty question field, got %q", stored["question"])
	}

	manager.rdb.HSet(context.Background(), "room:bad", map[string]interface{}{
		"matchId":          "bad",
		"user1":            "u1",
		"user2":            "u2",
		"category":         "cat",
		"difficulty":       "easy",
		"status":           "ready",
		"question":         "{invalid",
		"rerollsRemaining": "1",
		"createdAt":        "now",
	})
	if info, err := manager.fetchRoomStatusFromRedis("bad"); err != nil || info.Question != nil {
		t.Fatalf("expected graceful handling of bad question, got %#v err=%v", info, err)
	}
}

func TestGetRoomStatusCaches(t *testing.T) {
	manager, _, _ := setupRoomManager(t, nil)
	manager.roomStatusMap["cached"] = &models.RoomInfo{MatchId: "cached", Status: "ready"}

	info, err := manager.GetRoomStatus("cached")
	if err != nil || info.MatchId != "cached" {
		t.Fatalf("unexpected result: %#v err=%v", info, err)
	}
	if info == manager.roomStatusMap["cached"] {
		t.Fatalf("expected clone, got same pointer")
	}
}

func TestFetchRoomStatusFromRedisError(t *testing.T) {
	manager, _, _ := setupRoomManager(t, nil)
	_ = manager.rdb.Close()
	if _, err := manager.fetchRoomStatusFromRedis("missing"); err == nil {
		t.Fatalf("expected redis error")
	}
}

func TestGetRoomStatusLoadsFromRedis(t *testing.T) {
	manager, _, _ := setupRoomManager(t, nil)
	info := &models.RoomInfo{MatchId: "remote", Status: "ready"}
	manager.updateRoomStatusInRedis(context.Background(), info)

	loaded, err := manager.GetRoomStatus("remote")
	if err != nil || loaded.MatchId != "remote" {
		t.Fatalf("expected remote room, got %#v err=%v", loaded, err)
	}
	manager.mu.RLock()
	_, exists := manager.roomStatusMap["remote"]
	manager.mu.RUnlock()
	if !exists {
		t.Fatalf("expected room cached after load")
	}
}

func TestCloneRoomInfo(t *testing.T) {
	if cloneRoomInfo(nil) != nil {
		t.Fatalf("expected nil clone")
	}
	src := &models.RoomInfo{
		MatchId:  "m",
		Question: &models.Question{ID: 1},
	}
	dst := cloneRoomInfo(src)
	if dst == src || dst.Question == src.Question {
		t.Fatalf("expected deep copy")
	}
}

func TestRerollQuestionSuccessAndErrors(t *testing.T) {
	manager, _, server := setupRoomManager(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(models.Question{ID: 99})
	})

	manager.roomStatusMap["m"] = &models.RoomInfo{
		MatchId:          "m",
		Category:         "cat",
		Difficulty:       "easy",
		RerollsRemaining: 1,
		Question:         &models.Question{ID: 1},
		Status:           "ready",
	}

	updated, err := manager.RerollQuestion("m")
	if err != nil || updated.Question.ID != 99 {
		t.Fatalf("unexpected reroll result: %#v err=%v", updated, err)
	}

	manager.roomStatusMap["m"].RerollsRemaining = 0
	if _, err := manager.RerollQuestion("m"); !errors.Is(err, ErrNoRerolls) {
		t.Fatalf("expected ErrNoRerolls, got %v", err)
	}

	manager.roomStatusMap["x"] = &models.RoomInfo{
		MatchId:          "x",
		Category:         "cat",
		Difficulty:       "easy",
		RerollsRemaining: 1,
	}
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	})
	if _, err := manager.RerollQuestion("x"); err == nil {
		t.Fatalf("expected reroll error when fetch fails")
	}
}

func TestRerollQuestionMissingRoom(t *testing.T) {
	manager, _, _ := setupRoomManager(t, nil)
	if _, err := manager.RerollQuestion("missing"); err == nil {
		t.Fatalf("expected error when room missing")
	}
}

func TestRerollQuestionLoadsFromRedis(t *testing.T) {
	manager, _, _ := setupRoomManager(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(models.Question{ID: 42})
	})

	info := &models.RoomInfo{
		MatchId:          "remote",
		Category:         "cat",
		Difficulty:       "easy",
		RerollsRemaining: 1,
	}
	manager.updateRoomStatusInRedis(context.Background(), info)

	updated, err := manager.RerollQuestion("remote")
	if err != nil || updated.Question == nil || updated.Question.ID != 42 {
		t.Fatalf("expected reroll to load from redis, got %#v err=%v", updated, err)
	}
}

func TestValidateRoomAccess(t *testing.T) {
	manager, _, _ := setupRoomManager(t, nil)
	manager.roomStatusMap["match"] = &models.RoomInfo{
		MatchId: "match",
		User1:   "u1",
		User2:   "u2",
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &utils.RoomTokenClaims{
		MatchId: "match",
		UserId:  "u1",
	}).SignedString([]byte("your-secret-key"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	info, err := manager.ValidateRoomAccess(token)
	if err != nil || info.MatchId != "match" {
		t.Fatalf("unexpected validation result: %#v err=%v", info, err)
	}

	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, &utils.RoomTokenClaims{
		MatchId: "match",
		UserId:  "other",
	}).SignedString([]byte("your-secret-key"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := manager.ValidateRoomAccess(token); err == nil {
		t.Fatalf("expected unauthorized user error")
	}

	if _, err := manager.ValidateRoomAccess("bad.token"); err == nil {
		t.Fatalf("expected invalid token error")
	}
}

func TestValidateRoomAccessMissingRoom(t *testing.T) {
	manager, _, _ := setupRoomManager(t, nil)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &utils.RoomTokenClaims{MatchId: "missing", UserId: "u"}).SignedString([]byte("your-secret-key"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := manager.ValidateRoomAccess(token); err == nil {
		t.Fatalf("expected room not found error")
	}
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}
