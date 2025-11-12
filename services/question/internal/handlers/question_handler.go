package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"peerprep/question/internal/models"
	"peerprep/question/internal/utils"

	"github.com/go-chi/chi/v5"
)

type QuestionRepo interface {
	GetAll() ([]models.Question, error)
	GetAllWithPagination(page, limit int, search string) ([]models.Question, int, error)
	Create(*models.Question) (*models.Question, error)
	GetByID(int) (*models.Question, error)
	Update(int, *models.Question) (*models.Question, error)
	Delete(int) error
	GetRandom([]string, string) (*models.Question, error)
}

type QuestionHandler struct {
	repo QuestionRepo
}

func NewQuestionHandler(r QuestionRepo) *QuestionHandler {
	return &QuestionHandler{repo: r}
}

func (handler *QuestionHandler) GetQuestionsHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	// pagination parameters
	pageStr := request.URL.Query().Get("page")
	limitStr := request.URL.Query().Get("limit")
	search := request.URL.Query().Get("search")

	// If no pagination parameters, return all questions
	if pageStr == "" && limitStr == "" {
		questions, err := handler.repo.GetAll()
		if err != nil {
			utils.JSON(writer, http.StatusInternalServerError, models.ErrorResponse{
				Code:    "internal_error",
				Message: "Failed to fetch questions",
			})
			return
		}

		response := models.QuestionsResponse{
			Total:      len(questions),
			Items:      questions,
			Page:       1,
			Limit:      len(questions),
			TotalPages: 1,
			HasNext:    false,
			HasPrev:    false,
		}

		utils.JSON(writer, http.StatusOK, response)
		return
	}

	// default values
	page := 1
	limit := 10

	// parse page parameter
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		} else {
			utils.JSON(writer, http.StatusBadRequest, models.ErrorResponse{
				Code:    "invalid_page",
				Message: "page must be a positive integer",
			})
			return
		}
	}

	// parse limit parameter
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		} else {
			utils.JSON(writer, http.StatusBadRequest, models.ErrorResponse{
				Code:    "invalid_limit",
				Message: "limit must be a positive integer between 1 and 100",
			})
			return
		}
	}

	// questions with pagination and search
	questions, total, err := handler.repo.GetAllWithPagination(page, limit, search)
	if err != nil {
		utils.JSON(writer, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "internal_error",
			Message: "Failed to fetch questions",
		})
		return
	}

	// check if no questions found
	if total == 0 {
		if search != "" {
			utils.JSON(writer, http.StatusNotFound, models.ErrorResponse{
				Code:    "no_results",
				Message: "No questions found matching your search criteria",
			})
		} else {
			utils.JSON(writer, http.StatusNotFound, models.ErrorResponse{
				Code:    "no_questions",
				Message: "No questions available",
			})
		}
		return
	}

	// pagination metadata
	totalPages, hasNext, hasPrev := models.CalculatePaginationMeta(page, limit, total)

	response := models.QuestionsResponse{
		Total:      total,
		Items:      questions,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
	}

	utils.JSON(writer, http.StatusOK, response)
}

func (handler *QuestionHandler) CreateQuestionHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	var question models.Question
	if err := json.NewDecoder(request.Body).Decode(&question); err != nil {
		utils.JSON(writer, http.StatusBadRequest, models.ErrorResponse{
			Code:    "invalid_request",
			Message: "Invalid request payload",
		})
		return
	}

	created, err := handler.repo.Create(&question)
	if err != nil {
		utils.JSON(writer, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "internal_error",
			Message: "Failed to create question",
		})
		return
	}

	writer.Header().Set("Location", "/questions/"+strconv.Itoa(created.ID))
	utils.JSON(writer, http.StatusCreated, created)
}

func (handler *QuestionHandler) GetQuestionByIDHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	idStr := chi.URLParam(request, "id")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.JSON(writer, http.StatusBadRequest, models.ErrorResponse{
			Code:    "invalid_id",
			Message: "Invalid question ID",
		})
		return
	}

	question, err := handler.repo.GetByID(id)
	if err != nil {
		utils.JSON(writer, http.StatusNotFound, models.ErrorResponse{
			Code:    "question_not_found",
			Message: "Question not found",
		})
		return
	}

	utils.JSON(writer, http.StatusOK, question)
}

func (handler *QuestionHandler) UpdateQuestionHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	idStr := chi.URLParam(request, "id")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.JSON(writer, http.StatusBadRequest, models.ErrorResponse{
			Code:    "invalid_id",
			Message: "Invalid question ID",
		})
		return
	}

	var question models.Question
	if err := json.NewDecoder(request.Body).Decode(&question); err != nil {
		utils.JSON(writer, http.StatusBadRequest, models.ErrorResponse{
			Code:    "invalid_request",
			Message: "Invalid request payload",
		})
		return
	}

	updated, err := handler.repo.Update(id, &question)
	if err != nil {
		utils.JSON(writer, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "internal_error",
			Message: "Failed to update question",
		})
		return
	}

	utils.JSON(writer, http.StatusOK, updated)
}

func (handler *QuestionHandler) DeleteQuestionHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	idStr := chi.URLParam(request, "id")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.JSON(writer, http.StatusBadRequest, models.ErrorResponse{
			Code:    "invalid_id",
			Message: "Invalid question ID",
		})
		return
	}

	if err := handler.repo.Delete(id); err != nil {
		if err.Error() == "question not found" {
			utils.JSON(writer, http.StatusNotFound, models.ErrorResponse{
				Code:    "question_not_found",
				Message: "Question not found",
			})
			return
		}

		utils.JSON(writer, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "internal_error",
			Message: "Failed to delete question",
		})
		return
	}

	utils.JSON(writer, http.StatusOK, map[string]string{
		"message": "Question deleted successfully",
	})
}

func (handler *QuestionHandler) GetRandomQuestionHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	// parse query parameters
	difficulty := request.URL.Query().Get("difficulty")
	topicParam := request.URL.Query().Get("topic")

	fmt.Println("Difficulty: ", difficulty)
	fmt.Println("Topic: ", topicParam)

	// validate difficulty if provided
	d := strings.ToLower(difficulty)

	if d != "" {
		if d != strings.ToLower(string(models.Easy)) &&
			d != strings.ToLower(string(models.Medium)) &&
			d != strings.ToLower(string(models.Hard)) {
			fmt.Println("Invalid difficulty: ", difficulty)
			utils.JSON(writer, http.StatusBadRequest, models.ErrorResponse{
				Code:    "invalid_difficulty",
				Message: "difficulty must be one of: Easy, Medium, Hard",
			})
			return
		}
	}

	// parse topics (comma separated)
	var topics []string
	if topicParam != "" {
		topics = strings.Split(topicParam, ",")
		// trim whitespace from each topic
		for i, topic := range topics {
			topics[i] = strings.TrimSpace(topic)
		}
	}

	question, err := handler.repo.GetRandom(topics, difficulty)
	if err != nil {
		utils.JSON(writer, http.StatusNotFound, models.ErrorResponse{
			Code:    "no_eligible_question",
			Message: "no eligible question found",
		})
		return
	}

	utils.JSON(writer, http.StatusOK, question)
}
