package mongo

import (
	"context"
	"errors"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Question model 
type Question struct {
	ID             string     `json:"id"`         
	Title          string     `json:"title"`     
	Difficulty     Difficulty `json:"difficulty"`
	TopicTags      []string   `json:"topic_tags,omitempty" validate:"max=10"`
	PromptMarkdown string     `json:"prompt_markdown"`
	Constraints    string     `json:"constraints,omitempty"`
	TestCases      []TestCase `json:"test_cases,omitempty"`
	ImageURLs      []string   `json:"image_urls,omitempty" validate:"max=5"` 

	Status           Status     `json:"status,omitempty"` 
	Author           string     `json:"author,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeprecatedAt     *time.Time `json:"deprecated_at,omitempty"`
	DeprecatedReason string     `json:"deprecated_reason,omitempty"`
}

// Repo wraps a MongoDB collection
type Repo struct{ col *mongo.Collection }

// NewQuestionRepo connects to Mongo and ensures an index on Title
func NewQuestionRepo(c *Client) (*Repo, error) {
	db, err := c.DB()
	if err != nil {
		return nil, err
	}

	colName := os.Getenv("QUESTIONS_COLLECTION")
	if colName == "" {
		colName = "questions"
	}

	col := db.Collection(colName)
	r := &Repo{col: col}

	// Add a unique index on Title
	_, _ = r.col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    map[string]interface{}{"Title": 1},
		Options: options.Index().SetUnique(true),
	})

	return r, nil
}

// Create inserts a new question
func (r *Repo) Create(ctx context.Context, q *Question) (*Question, error) {
	if q.Title == "" {
		return nil, errors.New("title required")
	}
	now := time.Now().UTC()
	q.CreatedAt, q.UpdatedAt = now, now
	if q.Status == "" {
		q.Status = "active"
	}
	_, err := r.col.InsertOne(ctx, q)
	if err != nil {
		return nil, err
	}
	return q, nil
}

// List retrieves all active questions (limit optional)
func (r *Repo) List(ctx context.Context, limit int64) ([]Question, error) {
	opts := options.Find().SetLimit(limit)
	cur, err := r.col.Find(ctx, map[string]string{"Status": "active"}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []Question
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetByTitle retrieves a question by its title
func (r *Repo) GetByTitle(ctx context.Context, title string) (*Question, error) {
	var q Question
	if err := r.col.FindOne(ctx, map[string]string{"Title": title}).Decode(&q); err != nil {
		return nil, err
	}
	return &q, nil
}

// Update modifies an existing question by title
func (r *Repo) Update(ctx context.Context, title string, patch map[string]interface{}) (*Question, error) {
	patch["UpdatedAt"] = time.Now().UTC()
	var updated Question
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	if err := r.col.FindOneAndUpdate(ctx, map[string]string{"Title": title}, map[string]interface{}{"$set": patch}, opts).Decode(&updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

// Delete removes a question by title
func (r *Repo) Delete(ctx context.Context, title string) error {
	_, err := r.col.DeleteOne(ctx, map[string]string{"Title": title})
	return err
}
