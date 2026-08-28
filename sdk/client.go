package sdk

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"mtrx/internal/events"
)

type Client struct {
	BaseURL string
	Client_ *http.Client
}

func New(baseURL string) *Client {
	return &Client{BaseURL: baseURL, Client_: &http.Client{Timeout: 5 * time.Second}}
}

func (c *Client) Emit(e events.Event) error {
	if e.ID == "" {
		e.ID = events.NewID()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	b, _ := json.Marshal(e)
	_, err := c.Client_.Post(c.BaseURL+"/api/v1/events", "application/json", bytes.NewReader(b))
	return err
}
