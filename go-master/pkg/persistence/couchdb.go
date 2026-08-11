package persistence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type CouchDBClient struct {
	baseURL  string
	username string
	password string
	database string
	client   *http.Client
}

func NewCouchDBClient() (*CouchDBClient, error) {
	client := &CouchDBClient{
		baseURL:  getEnv("COUCHDB_URL", "http://localhost:5984"),
		username: os.Getenv("COUCHDB_USER"),
		password: os.Getenv("COUCHDB_PASSWORD"),
		database: getEnv("COUCHDB_DATABASE", "cloudai"),
		client:   &http.Client{Timeout: 10 * time.Second},
	}

	if err := client.CreateDatabase(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *CouchDBClient) CreateDatabase() error {
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/%s", c.baseURL, c.database), nil)
	c.setAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 201 = created, 412 = already exists
	if resp.StatusCode != 201 && resp.StatusCode != 412 {
		return fmt.Errorf("failed to create database: %s", resp.Status)
	}
	return nil
}

func (c *CouchDBClient) Put(ctx context.Context, docID string, doc interface{}) error {
	data, _ := json.Marshal(doc)
	req, _ := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/%s/%s", c.baseURL, c.database, docID),
		bytes.NewReader(data))

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 202 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("put failed: %s", body)
	}
	return nil
}

func (c *CouchDBClient) Get(ctx context.Context, docID string, result interface{}) error {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/%s/%s", c.baseURL, c.database, docID), nil)
	c.setAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("document not found")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("get failed: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

func (c *CouchDBClient) setAuth(req *http.Request) {
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
