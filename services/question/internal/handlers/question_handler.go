package handlers

import (
	"encoding/json"
	"net/http"

	"peerprep/question/internal/models"
	"peerprep/question/internal/repositories"
	"peerprep/question/internal/utils"

	"github.com/go-chi/chi/v5"
)

type QuestionHandler struct {
	repository *repositories.QuestionRepository
}

func NewQuestionHandler(repository *repositories.QuestionRepository) *QuestionHandler {
	return &QuestionHandler{repository: repository}
}

func (handler *QuestionHandler) GetQuestionsHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	questions, err := handler.repository.GetAll()
	if err != nil {
		utils.JSON(writer, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "internal_error",
			Message: "Failed to fetch questions",
		})
		return
	}

	response := models.QuestionsResponse{
		Total: len(questions),
		Items: questions,
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

	created, err := handler.repository.Create(&question)
	if err != nil {
		utils.JSON(writer, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "internal_error",
			Message: "Failed to create question",
		})
		return
	}

	writer.Header().Set("Location", "/questions/"+created.ID)
	utils.JSON(writer, http.StatusCreated, created)
}

func (handler *QuestionHandler) GetQuestionByIDHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	id := chi.URLParam(request, "id")

	question, err := handler.repository.GetByID(id)
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
	id := chi.URLParam(request, "id")

	var question models.Question
	if err := json.NewDecoder(request.Body).Decode(&question); err != nil {
		utils.JSON(writer, http.StatusBadRequest, models.ErrorResponse{
			Code:    "invalid_request",
			Message: "Invalid request payload",
		})
		return
	}

	updated, err := handler.repository.Update(id, &question)
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
	id := chi.URLParam(request, "id")

	if err := handler.repository.Delete(id); err != nil {
		utils.JSON(writer, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "internal_error",
			Message: "Failed to delete question",
		})
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (handler *QuestionHandler) GetRandomQuestionHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	_, err := handler.repository.GetRandom()
	if err != nil {
		utils.JSON(writer, http.StatusNotFound, models.ErrorResponse{
			Code:    "no_eligible_question",
			Message: "no eligible question found",
		})
		return
	}
}