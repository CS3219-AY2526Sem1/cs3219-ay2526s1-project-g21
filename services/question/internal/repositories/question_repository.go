package repositories

import (
	"errors"
	"time"

	"peerprep/question/internal/models"
)

// TODO: update all the methods to interact with the actual database

type QuestionRepository struct {
	// TODO: add db connection here ?????
}

// NewQuestionRepository creates a new instance of QuestionRepository
// We use the instance in the future to interact with the database
func NewQuestionRepository() *QuestionRepository {
	return &QuestionRepository{}
}

func (r *QuestionRepository) GetAll() ([]models.Question, error) {
	// return empty slice (current stub behavior)
	return []models.Question{}, nil
}

func (r *QuestionRepository) GetByID(id string) (*models.Question, error) {
	// return stub question with provided id for now
	current_time := time.Now().UTC()
	question := &models.Question{
		ID:             id,
		Title:          "stub",
		Difficulty:     models.Medium,
		TopicTags:      []string{"Stub"},
		PromptMarkdown: "stub prompt",
		Constraints:    "",
		TestCases:      []models.TestCase{{Input: "1", Output: "1"}},
		Status:         models.StatusActive,
		Author:         "system",
		CreatedAt:      current_time,
		UpdatedAt:      current_time,
	}
	return question, nil
}

func (r *QuestionRepository) Create(question *models.Question) (*models.Question, error) {
	// return stub created question
	current_time := time.Now().UTC()
	created := &models.Question{
		ID:             "stub-id",
		Title:          "stub",
		Difficulty:     models.Easy,
		TopicTags:      []string{"Stub"},
		PromptMarkdown: "stub prompt",
		Constraints:    "",
		TestCases:      []models.TestCase{{Input: "1", Output: "1"}},
		Status:         models.StatusActive,
		Author:         "system",
		CreatedAt:      current_time,
		UpdatedAt:      current_time,
	}
	return created, nil
}

func (r *QuestionRepository) Update(id string, question *models.Question) (*models.Question, error) {
	// return stub updated question
	current_time := time.Now().UTC()
	updated := &models.Question{
		ID:             id,
		Title:          "stub-updated",
		Difficulty:     models.Hard,
		TopicTags:      []string{"Stub"},
		PromptMarkdown: "stub prompt updated",
		Constraints:    "",
		TestCases:      []models.TestCase{{Input: "1", Output: "1"}},
		Status:         models.StatusActive,
		Author:         "system",
		CreatedAt:      current_time.Add(-time.Hour),
		UpdatedAt:      current_time,
	}
	return updated, nil
}

func (r *QuestionRepository) Delete(id string) error {
	// stub implementation - always succeeds
	return nil
}

func (r *QuestionRepository) GetRandom() (*models.Question, error) {
	// return error to match current stub behavior
	return nil, errors.New("no eligible question found")
}
