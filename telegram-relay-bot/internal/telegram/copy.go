package telegram

import (
	"fmt"
	"time"

	"gopkg.in/telebot.v3"
)

// Client wraps telebot.Bot with helper methods
type Client struct {
	*telebot.Bot
}

// NewClient creates a new Telegram client
func NewClient(token string, settings telebot.Settings) (*Client, error) {
	bot, err := telebot.NewBot(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}
	return &Client{Bot: bot}, nil
}

// CopyMessageToTopic copies a message to a specific topic in a supergroup
func (c *Client) CopyMessageToTopic(chatID int64, topicID int64, fromChatID int64, messageID int) (*telebot.Message, error) {
	dst := &telebot.Chat{ID: chatID}
	opts := &telebot.SendOptions{MessageThreadID: int(topicID)}
	msg, err := c.Copy(dst, fromChatID, messageID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to copy message: %w", err)
	}
	return msg, nil
}

// CopyMessageToUser copies a message from a group topic to a user
func (c *Client) CopyMessageToUser(userID int64, fromChatID int64, messageID int) (*telebot.Message, error) {
	dst := &telebot.Chat{ID: userID}
	msg, err := c.Copy(dst, fromChatID, messageID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to copy message: %w", err)
	}
	return msg, nil
}

// CreateTopic creates a new topic in a supergroup
func (c *Client) CreateTopic(chatID int64, name string) (int64, error) {
	chat := &telebot.Chat{ID: chatID}
	topicID, err := c.CreateTopic(chat, name)
	if err != nil {
		return 0, fmt.Errorf("failed to create topic: %w", err)
	}
	return int64(topicID), nil
}

// CloseTopic closes a topic in a supergroup
func (c *Client) CloseTopic(chatID int64, topicID int64) error {
	chat := &telebot.Chat{ID: chatID}
	err := c.CloseTopic(chat, int(topicID))
	if err != nil {
		return fmt.Errorf("failed to close topic: %w", err)
	}
	return nil
}

// ReopenTopic reopens a closed topic
func (c *Client) ReopenTopic(chatID int64, topicID int64) error {
	chat := &telebot.Chat{ID: chatID}
	err := c.ReopenTopic(chat, int(topicID))
	if err != nil {
		return fmt.Errorf("failed to reopen topic: %w", err)
	}
	return nil
}

// DeleteTopic deletes a topic from a supergroup
func (c *Client) DeleteTopic(chatID int64, topicID int64) error {
	chat := &telebot.Chat{ID: chatID}
	err := c.DeleteTopic(chat, int(topicID))
	if err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}
	return nil
}

// EditMessageText edits a text message
func (c *Client) EditMessageText(chat *telebot.Chat, messageID int, text string) error {
	_, err := c.Edit(chat, messageID, text)
	if err != nil {
		return fmt.Errorf("failed to edit message text: %w", err)
	}
	return nil
}

// EditMessageCaption edits a message caption (for media)
func (c *Client) EditMessageCaption(chat *telebot.Chat, messageID int, caption string) error {
	msg, err := c.GetMessage(*chat, messageID)
	if err != nil {
		return fmt.Errorf("failed to get message for caption edit: %w", err)
	}

	editable := &telebot.Message{
		Chat:    *chat,
		ID:      messageID,
		Caption: caption,
	}

	if msg.Photo != nil && len(msg.Photo) > 0 {
		editable.Photo = msg.Photo[0]
	} else if msg.Video != nil {
		editable.Video = msg.Video
	} else if msg.Audio != nil {
		editable.Audio = msg.Audio
	} else if msg.Document != nil {
		editable.Document = msg.Document
	} else if msg.Voice != nil {
		editable.Voice = msg.Voice
	} else if msg.Animation != nil {
		editable.Animation = msg.Animation
	}

	_, err = c.Edit(editable, editable.Caption)
	if err != nil {
		return fmt.Errorf("failed to edit message caption: %w", err)
	}
	return nil
}

// DeleteMessage deletes a message
func (c *Client) DeleteMessage(chat *telebot.Chat, messageID int) error {
	err := c.Delete(*chat, messageID)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}

// SendMessageWithInlineKeyboard sends a message with inline keyboard
func (c *Client) SendMessageWithInlineKeyboard(chat *telebot.Chat, text string, markup *telebot.ReplyMarkup) (*telebot.Message, error) {
	msg, err := c.Send(chat, text, &telebot.SendOptions{
		ReplyMarkup: markup,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}
	return msg, nil
}

// IsRetryableError checks if an error is retryable (rate limit, etc.)
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	if contains(errStr, "rate limit") || contains(errStr, "Too Many Requests") {
		return true
	}
	
	if contains(errStr, "timeout") || contains(errStr, "connection reset") {
		return true
	}
	
	return false
}

// RetryWithBackoff retries a function with exponential backoff
func RetryWithBackoff(maxRetries int, initialDelay time.Duration, fn func() error) error {
	var err error
	delay := initialDelay
	
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		
		if !IsRetryableError(err) {
			return err
		}
		
		time.Sleep(delay)
		delay *= 2
	}
	
	return err
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
