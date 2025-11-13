package repositories

import (
	"errors"
	"fmt"
	"testing"

	"peerprep/user/internal/models"
	"peerprep/user/internal/testhelpers"
)

func newRepo(t *testing.T) *UserRepository {
	t.Helper()
	return &UserRepository{DB: testhelpers.SetupTestDB(t)}
}

func TestUserRepository_CreateUser(t *testing.T) {
	repo := newRepo(t)

	user := &models.User{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
	}

	if err := repo.CreateUser(user); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if user.ID == 0 {
		t.Fatalf("expected user ID to be set")
	}
}

func TestUserRepository_GetUserByID(t *testing.T) {
	repo := newRepo(t)
	user := &models.User{Username: "bob", Email: "bob@example.com", PasswordHash: "hash"}
	if err := repo.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		got, err := repo.GetUserByID(fmt.Sprintf("%d", user.ID))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Email != user.Email {
			t.Fatalf("expected email %q, got %q", user.Email, got.Email)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		if _, err := repo.GetUserByID("abc"); err == nil {
			t.Fatalf("expected error for invalid id")
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := repo.GetUserByID("999"); err != ErrUserNotFound {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestUserRepository_GetUserByUsername(t *testing.T) {
	repo := newRepo(t)
	user := &models.User{Username: "charlie", Email: "charlie@example.com", PasswordHash: "hash"}
	if err := repo.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		got, err := repo.GetUserByUsername("CHARLIE")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != user.ID {
			t.Fatalf("expected id %d, got %d", user.ID, got.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := repo.GetUserByUsername("nobody"); err != ErrUserNotFound {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("database error", func(t *testing.T) {
		testhelpers.DropUserTable(t, repo.DB)
		if _, err := repo.GetUserByUsername("any"); err == nil || err == ErrUserNotFound {
			t.Fatalf("expected underlying DB error, got %v", err)
		}
	})
}

func TestUserRepository_GetUserByEmail(t *testing.T) {
	repo := newRepo(t)
	user := &models.User{Username: "david", Email: "david@example.com", PasswordHash: "hash"}
	if err := repo.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		got, err := repo.GetUserByEmail("DAVID@example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != user.ID {
			t.Fatalf("expected id %d, got %d", user.ID, got.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := repo.GetUserByEmail("none@example.com"); err != ErrUserNotFound {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("database error", func(t *testing.T) {
		testhelpers.DropUserTable(t, repo.DB)
		if _, err := repo.GetUserByEmail("any"); err == nil || err == ErrUserNotFound {
			t.Fatalf("expected underlying DB error, got %v", err)
		}
	})
}

func TestUserRepository_UpdateUser(t *testing.T) {
	repo := newRepo(t)
	original := &models.User{Username: "eve", Email: "eve@example.com", PasswordHash: "hash"}
	if err := repo.CreateUser(original); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		updates := &models.User{Email: "eve2@example.com"}
		updated, err := repo.UpdateUser(fmt.Sprintf("%d", original.ID), updates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Email != updates.Email {
			t.Fatalf("expected email %q, got %q", updates.Email, updated.Email)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		if _, err := repo.UpdateUser("abc", &models.User{}); err == nil {
			t.Fatalf("expected error for invalid id")
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := repo.UpdateUser("999", &models.User{}); err != ErrUserNotFound {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("load error", func(t *testing.T) {
		repo := newRepo(t)
		original := &models.User{Username: "ivy", Email: "ivy@example.com", PasswordHash: "hash"}
		if err := repo.CreateUser(original); err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}
		testhelpers.DropUserTable(t, repo.DB)
		if _, err := repo.UpdateUser(fmt.Sprintf("%d", original.ID), &models.User{Email: "new@example.com"}); err == nil || errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected underlying DB error, got %v", err)
		}
	})

	t.Run("update error", func(t *testing.T) {
		other := &models.User{Username: "frank", Email: "frank@example.com", PasswordHash: "hash"}
		if err := repo.CreateUser(other); err != nil {
			t.Fatalf("failed to seed second user: %v", err)
		}

		if _, err := repo.UpdateUser(fmt.Sprintf("%d", original.ID), &models.User{Email: other.Email}); err == nil {
			t.Fatalf("expected update to fail due to unique constraint")
		}
	})
}

func TestUserRepository_DeleteUser(t *testing.T) {
	repo := newRepo(t)
	user := &models.User{Username: "grace", Email: "grace@example.com", PasswordHash: "hash"}
	if err := repo.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	t.Run("invalid id", func(t *testing.T) {
		if err := repo.DeleteUser("abc"); err == nil {
			t.Fatalf("expected error for invalid id")
		}
	})

	t.Run("not found", func(t *testing.T) {
		if err := repo.DeleteUser("999"); err != ErrUserNotFound {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		repo := newRepo(t)
		user := &models.User{Username: "henry", Email: "henry@example.com", PasswordHash: "hash"}
		if err := repo.CreateUser(user); err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}
		if err := repo.DeleteUser(fmt.Sprintf("%d", user.ID)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
