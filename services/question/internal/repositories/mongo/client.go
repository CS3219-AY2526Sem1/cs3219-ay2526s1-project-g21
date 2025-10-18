package mongo

import (
	"context"
	"errors"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Client struct{ raw *mongo.Client }

func NewClient(ctx context.Context) (*Client, error) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return nil, errors.New("MONGO_URI is empty")
	}
	c, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}
	return &Client{raw: c}, nil
}

func (c *Client) DB() (*mongo.Database, error) {
	if c == nil || c.raw == nil {
		return nil, errors.New("mongo client not initialized")
	}
	name := os.Getenv("QUESTIONS_DB_NAME")
	if name == "" {
		name = "peerprep"
	}
	return c.raw.Database(name), nil
}
