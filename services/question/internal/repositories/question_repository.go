package repositories

import (
	"context"
	"errors"
	"os"
	"time"

	"peerprep/question/internal/models"
	mongoclient "peerprep/question/internal/repositories/mongo"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TODO: update all the methods to interact with the actual database

type QuestionRepository struct {
	col *mongo.Collection
}

// Creates a new MongoDB-backed repository
// We use the instance in the future to interact with the database

func NewQuestionRepository(ctx context.Context) (*QuestionRepository, error) {
	client, err := mongoclient.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	db, err := client.DB()
	if err != nil {
		return nil, err
	}

	colName := os.Getenv("QUESTIONS_COLLECTION")
	if colName == "" {
		colName = "questions"
	}

	col := db.Collection(colName)

	// ensure Title is unique
	_, _ = col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.M{"title": 1},
		Options: options.Index().SetUnique(true),
	})

	return &QuestionRepository{col: col}, nil
}

func (r *QuestionRepository) GetAll() ([]models.Question, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cur, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var results []models.Question
	if err := cur.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *QuestionRepository) GetByID(id string) (*models.Question, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var q models.Question
	err := r.col.FindOne(ctx, bson.M{"id": id}).Decode(&q)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *QuestionRepository) Create(question *models.Question) (*models.Question, error) {
	if question.Title == "" {
		return nil, errors.New("title required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	question.CreatedAt, question.UpdatedAt = now, now

	_, err := r.col.InsertOne(ctx, question)
	if err != nil {
		return nil, err
	}
	return question, nil
}

func (r *QuestionRepository) Update(id string, question *models.Question) (*models.Question, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	question.UpdatedAt = time.Now().UTC()

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.Question
	err := r.col.FindOneAndUpdate(ctx, bson.M{"id": id}, bson.M{"$set": question}, opts).Decode(&updated)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *QuestionRepository) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.col.DeleteOne(ctx, bson.M{"id": id})
	return err
}

func (r *QuestionRepository) GetRandom() (*models.Question, error) {
	// return error to match current stub behavior
	return nil, errors.New("no eligible question found")
}
