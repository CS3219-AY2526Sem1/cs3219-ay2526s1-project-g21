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

	// ensure ID is unique
	_, _ = col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.M{"id": 1},
		Options: options.Index().SetUnique(true),
	})

	// ensure Title is unique
	_, _ = col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.M{"title": 1},
		Options: options.Index().SetUnique(true),
	})

	return &QuestionRepository{col: col}, nil
}

// Get all questions
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

// Get question by ID
func (r *QuestionRepository) GetByID(id int) (*models.Question, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var q models.Question
	err := r.col.FindOne(ctx, bson.M{"id": id}).Decode(&q)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// Create a new question
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

// Update an existing question
func (r *QuestionRepository) Update(id int, question *models.Question) (*models.Question, error) {
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

// Delete a question by ID
func (r *QuestionRepository) Delete(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := r.col.DeleteOne(ctx, bson.M{"id": id})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return errors.New("question not found")
	}

	return nil
}

// Get a random question with optional filters
func (r *QuestionRepository) GetRandom(topics []string, difficulty string) (*models.Question, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// build match criteria
	matchCriteria := bson.M{"status": "active"}

	// add difficulty filter if provided
	if difficulty != "" {
		matchCriteria["difficulty"] = difficulty
	}

	// add topic filter if provided
	if len(topics) > 0 {
		matchCriteria["topic_tags"] = bson.M{"$in": topics}
	}

	// 1) only consider active questions with optional filters
	// 2) pick one random document
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: matchCriteria}},
		bson.D{{Key: "$sample", Value: bson.M{"size": 1}}},
	}

	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	// advance to the first document (if any)
	if !cursor.Next(ctx) {
		// check for cursor errors, otherwise report no docs found
		if cursorErr := cursor.Err(); cursorErr != nil {
			return nil, cursorErr
		}
		return nil, errors.New("no eligible question found")
	}

	var picked models.Question
	if err := cursor.Decode(&picked); err != nil {
		return nil, err
	}

	return &picked, nil
}
