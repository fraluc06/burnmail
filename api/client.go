package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const BaseURL = "https://api.mail.tm"

var (
	clientInstance *Client
	clientOnce     sync.Once
)

type Client struct {
	HTTPClient  *http.Client
	token       string
	mu          sync.RWMutex
	rateLimiter chan struct{}
	lastRequest time.Time
	minDelay    time.Duration
}

func GetClient() *Client {
	clientOnce.Do(func() {
		limiter := make(chan struct{}, 5)
		for i := 0; i < 5; i++ {
			limiter <- struct{}{}
		}

		clientInstance = &Client{
			HTTPClient: &http.Client{
				Timeout: 30 * time.Second,
				Transport: &http.Transport{
					MaxIdleConns:        10,
					MaxIdleConnsPerHost: 10,
					IdleConnTimeout:     90 * time.Second,
					DisableKeepAlives:   false,
					DisableCompression:  false,
				},
			},
			rateLimiter: limiter,
			minDelay:    200 * time.Millisecond,
		}
	})
	return clientInstance
}

func (c *Client) waitForRateLimit() {
	<-c.rateLimiter

	// Reserve the next request slot under the lock, then sleep outside it so
	// concurrent SetToken/GetToken calls are not blocked by the delay.
	c.mu.Lock()
	next := c.lastRequest.Add(c.minDelay)
	if now := time.Now(); next.Before(now) {
		next = now
	}
	c.lastRequest = next
	c.mu.Unlock()

	time.Sleep(time.Until(next))

	go func() {
		time.Sleep(c.minDelay)
		c.rateLimiter <- struct{}{}
	}()
}

func (c *Client) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

func (c *Client) GetToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

type Domain struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Account struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Message struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"accountId"`
	MsgID       string    `json:"msgid"`
	From        From      `json:"from"`
	To          []To      `json:"to"`
	Subject     string    `json:"subject"`
	Intro       string    `json:"intro"`
	Seen        bool      `json:"seen"`
	IsDeleted   bool      `json:"isDeleted"`
	HasAttach   bool      `json:"hasAttachments"`
	Size        int       `json:"size"`
	DownloadURL string    `json:"downloadUrl"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type From struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

type To struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

type Attachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
	DownloadURL string `json:"downloadUrl"`
}

type MessageDetail struct {
	Message
	CC            []any                  `json:"cc"`
	BCC           []any                  `json:"bcc"`
	Flagged       bool                   `json:"flagged"`
	Verifications map[string]interface{} `json:"verifications"`
	Retention     bool                   `json:"retention"`
	RetentionDate time.Time              `json:"retentionDate"`
	Text          string                 `json:"text"`
	HTML          []string               `json:"html"`
	Attachments   []Attachment           `json:"attachments"`
}

type AuthResponse struct {
	Token string `json:"token"`
	ID    string `json:"id"`
}

type hydraResponse struct {
	Member json.RawMessage `json:"hydra:member"`
}

func (c *Client) GetDomains() ([]Domain, error) {
	c.waitForRateLimit()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", BaseURL+"/domains", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get domains: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var domains []Domain
	if err := json.Unmarshal(body, &domains); err == nil {
		return domains, nil
	}

	var hydra hydraResponse
	if err := json.Unmarshal(body, &hydra); err != nil {
		return nil, fmt.Errorf("failed to parse domains response: %w", err)
	}

	if len(hydra.Member) > 0 {
		if err := json.Unmarshal(hydra.Member, &domains); err != nil {
			return nil, err
		}
		return domains, nil
	}

	return nil, errors.New("unexpected API response format")
}

func (c *Client) CreateAccount(address, password string) (*Account, error) {
	payload := map[string]string{
		"address":  address,
		"password": password,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Post(BaseURL+"/accounts", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create account: status %d, body: %s", resp.StatusCode, string(body))
	}

	var account Account
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, err
	}

	return &account, nil
}

func (c *Client) Login(address, password string) (string, error) {
	payload := map[string]string{
		"address":  address,
		"password": password,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := c.HTTPClient.Post(BaseURL+"/token", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to login: status %d, body: %s", resp.StatusCode, string(body))
	}

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", err
	}

	c.SetToken(authResp.Token)
	return authResp.Token, nil
}

func (c *Client) GetMessages() ([]Message, error) {
	c.waitForRateLimit()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", BaseURL+"/messages", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.GetToken())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get messages: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var messages []Message
	if err := json.Unmarshal(body, &messages); err == nil {
		return messages, nil
	}

	var hydra hydraResponse
	if err := json.Unmarshal(body, &hydra); err != nil {
		return nil, fmt.Errorf("failed to parse messages response: %w", err)
	}

	if len(hydra.Member) > 0 {
		if err := json.Unmarshal(hydra.Member, &messages); err != nil {
			return nil, err
		}
		return messages, nil
	}

	return []Message{}, nil
}

func (c *Client) GetMessage(id string) (*MessageDetail, error) {
	c.waitForRateLimit()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", BaseURL+"/messages/"+id, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.GetToken())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get message: status %d", resp.StatusCode)
	}

	var message MessageDetail
	if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
		return nil, err
	}

	return &message, nil
}

func (c *Client) DeleteAccount(accountID string) error {
	req, err := http.NewRequest("DELETE", BaseURL+"/accounts/"+accountID, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.GetToken())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete account: status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) GetAccount(accountID string) (*Account, error) {
	req, err := http.NewRequest("GET", BaseURL+"/accounts/"+accountID, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.GetToken())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get account: status %d", resp.StatusCode)
	}

	var account Account
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, err
	}

	return &account, nil
}

func (c *Client) DeleteMessage(id string) error {
	c.waitForRateLimit()

	req, err := http.NewRequest("DELETE", BaseURL+"/messages/"+id, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.GetToken())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete message: status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) MarkMessageAsRead(id string) error {
	c.waitForRateLimit()

	req, err := http.NewRequest("PATCH", BaseURL+"/messages/"+id, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.GetToken())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to mark message as read: status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) DownloadAttachment(messageID, attachmentID string) ([]byte, error) {
	c.waitForRateLimit()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/messages/%s/attachment/%s", BaseURL, messageID, attachmentID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.GetToken())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download attachment: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
