package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"peerprep/user/internal/models"
	"peerprep/user/internal/repositories"
	"peerprep/user/internal/testhelpers"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

func newUserHandlerWithDB(t *testing.T) (*UserHandler, *repositories.UserRepository, *models.User) {
	t.Helper()
	repo := &repositories.UserRepository{DB: testhelpers.SetupTestDB(t)}
	user := &models.User{Username: "user", Email: "user@example.com", PasswordHash: "hash"}
	if err := repo.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	h := &UserHandler{Repo: repo, JWTSecret: "test-secret"}
	return h, repo, user
}

func requestWithUserID(method, target, id string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	rctx := chi.NewRouteContext()
	if id != "" {
		rctx.URLParams.Add("id", id)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func TestUserHandler_UpdateUserHandler(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		handler, _, _ := newUserHandlerWithDB(t)
		req := requestWithUserID(http.MethodPut, "/users/1", "1", nil)
		rec := httptest.NewRecorder()

		handler.UpdateUserHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		handler, _, _ := newUserHandlerWithDB(t)
		req := requestWithUserID(http.MethodPut, "/users/1", "1", nil)
		token := makeToken(t, "other-secret", jwt.MapClaims{"sub": "1", "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.UpdateUserHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		handler, _, user := newUserHandlerWithDB(t)
		req := requestWithUserID(http.MethodPut, "/users/", "", nil)
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": fmt.Sprintf("%d", user.ID), "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.UpdateUserHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("forbidden update", func(t *testing.T) {
		handler, _, user := newUserHandlerWithDB(t)
		req := requestWithUserID(http.MethodPut, "/users/999", "999", bytes.NewBufferString(`{"email":"new@example.com"}`))
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": fmt.Sprintf("%d", user.ID), "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.UpdateUserHandler(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("invalid payload", func(t *testing.T) {
		handler, _, user := newUserHandlerWithDB(t)
		req := requestWithUserID(http.MethodPut, "/users/"+fmt.Sprint(user.ID), fmt.Sprint(user.ID), bytes.NewBufferString("{invalid"))
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": fmt.Sprintf("%d", user.ID), "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.UpdateUserHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		handler := &UserHandler{
			Repo: &mockUserRepo{
				getUserByIDFn: func(string) (*models.User, error) { return nil, nil },
				updateUserFn: func(string, *models.User) (*models.User, error) {
					return nil, repositories.ErrUserNotFound
				},
			},
			JWTSecret: "secret",
		}
		body := bytes.NewBufferString(`{"email":"new@example.com"}`)
		req := requestWithUserID(http.MethodPut, "/users/1", "1", body)
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": "1", "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.UpdateUserHandler(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("update failure", func(t *testing.T) {
		handler := &UserHandler{
			Repo: &mockUserRepo{
				updateUserFn: func(string, *models.User) (*models.User, error) { return nil, errors.New("db error") },
			},
			JWTSecret: "secret",
		}
		body := bytes.NewBufferString(`{"email":"new@example.com"}`)
		req := requestWithUserID(http.MethodPut, "/users/1", "1", body)
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": "1", "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.UpdateUserHandler(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		handler, repo, user := newUserHandlerWithDB(t)
		body := bytes.NewBufferString(`{"email":"updated@example.com"}`)
		req := requestWithUserID(http.MethodPut, fmt.Sprintf("/users/%d", user.ID), fmt.Sprintf("%d", user.ID), body)
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": fmt.Sprintf("%d", user.ID), "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.UpdateUserHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var updated models.User
		if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if updated.Email != "updated@example.com" {
			t.Fatalf("expected updated email, got %q", updated.Email)
		}
		stored, err := repo.GetUserByID(fmt.Sprintf("%d", user.ID))
		if err != nil {
			t.Fatalf("failed to load user: %v", err)
		}
		if stored.Email != "updated@example.com" {
			t.Fatalf("expected stored email to update, got %q", stored.Email)
		}
	})
}

func TestUserHandler_DeleteUserHandler(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		handler, _, _ := newUserHandlerWithDB(t)
		req := requestWithUserID(http.MethodDelete, "/users/1", "1", nil)
		rec := httptest.NewRecorder()

		handler.DeleteUserHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		handler, _, _ := newUserHandlerWithDB(t)
		req := requestWithUserID(http.MethodDelete, "/users/1", "1", nil)
		token := makeToken(t, "other-secret", jwt.MapClaims{"sub": "1", "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.DeleteUserHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		handler, _, user := newUserHandlerWithDB(t)
		req := requestWithUserID(http.MethodDelete, "/users/", "", nil)
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": fmt.Sprintf("%d", user.ID), "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.DeleteUserHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("forbidden delete", func(t *testing.T) {
		handler, _, user := newUserHandlerWithDB(t)
		req := requestWithUserID(http.MethodDelete, "/users/999", "999", nil)
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": fmt.Sprintf("%d", user.ID), "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.DeleteUserHandler(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		handler := &UserHandler{
			Repo: &mockUserRepo{
				deleteUserFn: func(string) error { return repositories.ErrUserNotFound },
			},
			JWTSecret: "secret",
		}
		req := requestWithUserID(http.MethodDelete, "/users/1", "1", nil)
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": "1", "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.DeleteUserHandler(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("delete failure", func(t *testing.T) {
		handler := &UserHandler{
			Repo: &mockUserRepo{
				deleteUserFn: func(string) error { return errors.New("db error") },
			},
			JWTSecret: "secret",
		}
		req := requestWithUserID(http.MethodDelete, "/users/1", "1", nil)
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": "1", "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.DeleteUserHandler(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		handler, repo, user := newUserHandlerWithDB(t)
		req := requestWithUserID(http.MethodDelete, fmt.Sprintf("/users/%d", user.ID), fmt.Sprintf("%d", user.ID), nil)
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{"sub": fmt.Sprintf("%d", user.ID), "exp": time.Now().Add(time.Hour).Unix()})
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.DeleteUserHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("expected empty body, got %q", rec.Body.String())
		}
		if err := repo.DeleteUser(fmt.Sprintf("%d", user.ID)); err != repositories.ErrUserNotFound {
			t.Fatalf("expected user to be deleted")
		}
	})
}
