// Package telegram provides a Telegram Bot API client for capturing group chat
// messages during D&D sessions.
package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"
)

// Message represents a captured Telegram message.
type Message struct {
	MessageID    int64
	FromID       int64
	FromUsername string
	FromDisplay  string
	Text         string
	Timestamp    time.Time
}

// Client wraps the Telegram Bot API.
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient creates a new Telegram Bot API client.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // long-polling timeout + buffer
		},
	}
}

// Listener captures messages from a specific chat via long-polling.
type Listener struct {
	client   *Client
	chatID   int64
	messages []Message
	mu       sync.Mutex
	cancel   context.CancelFunc
	done     chan struct{}
}

// StartListening begins long-polling for messages in the given chat.
// Call Stop() to end listening and retrieve captured messages.
func (c *Client) StartListening(ctx context.Context, chatID int64) *Listener {
	ctx, cancel := context.WithCancel(ctx)
	l := &Listener{
		client: c,
		chatID: chatID,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go l.poll(ctx)
	log.Printf("telegram: started listening on chat %d", chatID)
	return l
}

// Stop ends the listener and returns all captured messages.
func (l *Listener) Stop() []Message {
	l.cancel()
	<-l.done // wait for poll goroutine to exit
	l.mu.Lock()
	defer l.mu.Unlock()
	msgs := make([]Message, len(l.messages))
	copy(msgs, l.messages)
	log.Printf("telegram: stopped listening, captured %d messages", len(msgs))
	return msgs
}

// Messages returns a snapshot of currently captured messages.
func (l *Listener) Messages() []Message {
	l.mu.Lock()
	defer l.mu.Unlock()
	msgs := make([]Message, len(l.messages))
	copy(msgs, l.messages)
	return msgs
}

// poll runs the getUpdates long-polling loop.
func (l *Listener) poll(ctx context.Context) {
	defer close(l.done)

	offset := int64(0)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := l.client.getUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return // context cancelled
			}
			log.Printf("telegram: getUpdates error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			if u.Message == nil || u.Message.Chat.ID != l.chatID {
				continue
			}
			if u.Message.Text == "" {
				continue // skip non-text messages (photos, stickers, etc.)
			}

			msg := Message{
				MessageID:    int64(u.Message.MessageID),
				FromID:       u.Message.From.ID,
				FromUsername: u.Message.From.Username,
				FromDisplay:  u.Message.From.DisplayName(),
				Text:         u.Message.Text,
				Timestamp:    time.Unix(int64(u.Message.Date), 0),
			}

			l.mu.Lock()
			l.messages = append(l.messages, msg)
			l.mu.Unlock()
		}
	}
}

// IsRelevant returns true if a message is likely session-relevant content
// (not just chatter). Messages from the DM that are short one-liners,
// emoji-only, or trivial are filtered out.
func IsRelevant(msg Message, isDM bool) bool {
	text := msg.Text

	// Always include long messages (info dumps)
	if utf8.RuneCountInString(text) >= 50 {
		return true
	}

	// Skip very short messages (likely chatter)
	if utf8.RuneCountInString(text) < 20 {
		return false
	}

	// For medium-length messages, include if from DM
	return isDM
}

// Telegram Bot API types (minimal subset)

type apiResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	MessageID int    `json:"message_id"`
	From      *user  `json:"from"`
	Chat      chat   `json:"chat"`
	Date      int    `json:"date"`
	Text      string `json:"text"`
}

type user struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

func (u *user) DisplayName() string {
	if u.FirstName != "" && u.LastName != "" {
		return u.FirstName + " " + u.LastName
	}
	if u.FirstName != "" {
		return u.FirstName
	}
	if u.Username != "" {
		return u.Username
	}
	return fmt.Sprintf("user_%d", u.ID)
}

type chat struct {
	ID int64 `json:"id"`
}

func (c *Client) getUpdates(ctx context.Context, offset int64, timeout int) ([]update, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=%d&allowed_updates=[\"message\"]",
		c.token, offset, timeout)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !apiResp.OK {
		return nil, fmt.Errorf("telegram API error: %s", string(body))
	}

	var updates []update
	if err := json.Unmarshal(apiResp.Result, &updates); err != nil {
		return nil, fmt.Errorf("parse updates: %w", err)
	}

	return updates, nil
}
