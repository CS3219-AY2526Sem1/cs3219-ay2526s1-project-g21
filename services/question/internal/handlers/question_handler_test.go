package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"peerprep/question/internal/handlers"
	"peerprep/question/internal/models"
	"peerprep/question/internal/repositories"
)

type fakeRepo struct {
	getAllFn               func() ([]models.Question, error)
	getAllWithPaginationFn func(int, int, string) ([]models.Question, int, error)
	createFn               func(*models.Question) (*models.Question, error)
	getByIDFn              func(int) (*models.Question, error)
	updateFn               func(int, *models.Question) (*models.Question, error)
	deleteFn               func(int) error
	randomFn               func([]string, string) (*models.Question, error)
}

func (f *fakeRepo) GetAll() ([]models.Question, error) {
	if f.getAllFn != nil {
		return f.getAllFn()
	}
	return []models.Question{}, nil
}
func (f *fakeRepo) GetAllWithPagination(page, limit int, search string) ([]models.Question, int, error) {
	if f.getAllWithPaginationFn != nil {
		return f.getAllWithPaginationFn(page, limit, search)
	}
	return []models.Question{}, 0, repositories.ErrNotImplemented
}
func (f *fakeRepo) Create(q *models.Question) (*models.Question, error) {
	if f.createFn != nil {
		return f.createFn(q)
	}
	return nil, repositories.ErrNotImplemented
}
func (f *fakeRepo) GetByID(id int) (*models.Question, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(id)
	}
	return nil, repositories.ErrNotImplemented
}
func (f *fakeRepo) Update(id int, q *models.Question) (*models.Question, error) {
	if f.updateFn != nil {
		return f.updateFn(id, q)
	}
	return nil, repositories.ErrNotImplemented
}
func (f *fakeRepo) Delete(id int) error {
	if f.deleteFn != nil {
		return f.deleteFn(id)
	}
	return repositories.ErrNotImplemented
}
func (f *fakeRepo) GetRandom(topics []string, difficulty string) (*models.Question, error) {
	if f.randomFn != nil {
		return f.randomFn(topics, difficulty)
	}
	return nil, repositories.ErrNotImplemented
}

// Tests
//

// GET /questions
func TestGetQuestions_OK(t *testing.T) {
	repo := &fakeRepo{
		getAllFn: func() ([]models.Question, error) {
			return []models.Question{
				{ID: 1, Title: "Two Sum", Difficulty: models.Easy},
				{ID: 2, Title: "LRU Cache", Difficulty: models.Medium},
			}, nil
		},
	}
	h := handlers.NewQuestionHandler(repo)

	r := chi.NewRouter()
	r.Get("/api/v1/questions", h.GetQuestionsHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/questions", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var got models.QuestionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v\nbody=%s", err, rr.Body.String())
	}
	if got.Total != 2 || len(got.Items) != 2 {
		t.Fatalf("unexpected payload: %+v", got)
	}
	if got.Items[0].Title != "Two Sum" || got.Items[1].Title != "LRU Cache" {
		t.Fatalf("wrong items: %+v", got.Items)
	}
}

// POST /questions (valid)
func TestCreateQuestion_Valid(t *testing.T) {
	repo := &fakeRepo{
		createFn: func(q *models.Question) (*models.Question, error) {
			q.ID = 101
			return q, nil
		},
	}
	h := handlers.NewQuestionHandler(repo)

	r := chi.NewRouter()
	r.Post("/api/v1/questions", h.CreateQuestionHandler)

	body := bytes.NewBufferString(`{"title":"Two Sum","difficulty":"Easy","status":"active"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("expected 201/200, got %d: %s", rr.Code, rr.Body.String())
	}

	var created models.Question
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if created.ID == 0 || created.Title != "Two Sum" {
		t.Fatalf("unexpected created: %+v", created)
	}
}

// POST /questions (bad JSON)
func TestCreateQuestion_BadJSON(t *testing.T) {
	repo := &fakeRepo{} // createFn not used
	h := handlers.NewQuestionHandler(repo)

	r := chi.NewRouter()
	r.Post("/api/v1/questions", h.CreateQuestionHandler)

	// invalid: title should be string, not number
	body := bytes.NewBufferString(`{"title":123,"difficulty":"Easy"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// GET /questions/{id} (found)
func TestGetQuestionByID_Found(t *testing.T) {
	repo := &fakeRepo{
		getByIDFn: func(id int) (*models.Question, error) {
			return &models.Question{
				ID: id, Title: "Median of Two Sorted Arrays", Difficulty: models.Hard,
			}, nil
		},
	}
	h := handlers.NewQuestionHandler(repo)

	r := chi.NewRouter()
	r.Get("/api/v1/questions/{id}", h.GetQuestionByIDHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/questions/222", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var got models.Question
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if got.ID != 222 || got.Title == "" {
		t.Fatalf("unexpected question: %+v", got)
	}
}

// GET /questions/{id} (not found)
func TestGetQuestionByID_NotFound(t *testing.T) {
	repo := &fakeRepo{
		getByIDFn: func(id int) (*models.Question, error) {
			return nil, repositories.ErrNotFound
		},
	}
	h := handlers.NewQuestionHandler(repo)

	r := chi.NewRouter()
	r.Get("/api/v1/questions/{id}", h.GetQuestionByIDHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/questions/9999", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// PUT /questions/{id}
func TestUpdateQuestion_Valid(t *testing.T) {
	repo := &fakeRepo{
		updateFn: func(id int, q *models.Question) (*models.Question, error) {
			q.ID = id
			q.Title = "Two Sum (Updated)"
			return q, nil
		},
	}
	h := handlers.NewQuestionHandler(repo)

	r := chi.NewRouter()
	r.Put("/api/v1/questions/{id}", h.UpdateQuestionHandler)

	body := bytes.NewBufferString(`{"title":"Two Sum (Updated)","difficulty":"Medium","status":"active"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/questions/1", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var got models.Question
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if got.ID != 1 || got.Title != "Two Sum (Updated)" || got.Difficulty != models.Medium {
		t.Fatalf("unexpected updated: %+v", got)
	}
}

// DELETE /questions/{id}
func TestDeleteQuestion_OK(t *testing.T) {
	called := false
	repo := &fakeRepo{
		deleteFn: func(id int) error {
			called = true
			return nil
		},
	}
	h := handlers.NewQuestionHandler(repo)

	r := chi.NewRouter()
	r.Delete("/api/v1/questions/{id}", h.DeleteQuestionHandler)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/questions/123", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Fatalf("expected 200/204, got %d: %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatalf("expected delete to be called")
	}
}

// GET /questions/random?difficulty=Medium&topic=String,Sliding%20Window
func TestGetRandomQuestion_WithFilters(t *testing.T) {
	repo := &fakeRepo{
		randomFn: func(topics []string, difficulty string) (*models.Question, error) {
			if difficulty != "Medium" {
				return nil, repositories.ErrNotImplemented
			}
			return &models.Question{
				ID: 3, Title: "Longest Substring Without Repeating Characters",
				Difficulty: models.Medium, TopicTags: []string{"String", "Sliding Window"},
			}, nil
		},
	}
	h := handlers.NewQuestionHandler(repo)

	r := chi.NewRouter()
	r.Get("/api/v1/questions/random", h.GetRandomQuestionHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/questions/random?difficulty=Medium&topic=String,Sliding%20Window", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var got models.Question
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if got.ID == 0 || got.Difficulty != models.Medium {
		t.Fatalf("unexpected random question: %+v", got)
	}
}
