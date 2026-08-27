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

// CopyMessageToTopic copies a message from a client to a topic in the supergroup
// Uses bot.Copy with the incoming message as Editable
func (c *Client) CopyMessageToTopic(supergroupChat *telebot.Chat, topicID int64, incomingMsg *telebot.Message) (*telebot.Message, error) {
	msg, err := c.Copy(supergroupChat, incomingMsg, &telebot.SendOptions{ThreadID: int(topicID)})
	if err != nil {
		return nil, fmt.Errorf("failed to copy message: %w", err)
	}
	return msg, nil
}

// CopyMessageToUser copies a message from the owner in a topic to a client
// Uses bot.Copy with the incoming message as Editable
func (c *Client) CopyMessageToUser(clientChat *telebot.Chat, incomingMsg *telebot.Message) (*telebot.Message, error) {
	msg, err := c.Copy(clientChat, incomingMsg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to copy message: %w", err)
	}
	return msg, nil
}

// CreateTopic creates a new topic in a supergroup
func (c *Client) CreateTopic(chatID int64, name string) (int64, error) {
	chat := &telebot.Chat{ID: chatID}
	topic := &telebot.Topic{Name: name}
	result, err := c.Bot.CreateTopic(chat, topic)
	if err != nil {
		return 0, fmt.Errorf("failed to create topic: %w", err)
	}
	return int64(result.ThreadID), nil
}

// CloseTopic closes a topic in a supergroup
func (c *Client) CloseTopic(chatID int64, topicID int64) error {
	chat := &telebot.Chat{ID: chatID}
	topic := &telebot.Topic{ThreadID: int(topicID)}
	err := c.Bot.CloseTopic(chat, topic)
	if err != nil {
		return fmt.Errorf("failed to close topic: %w", err)
	}
	return nil
}

// ReopenTopic reopens a closed topic
func (c *Client) ReopenTopic(chatID int64, topicID int64) error {
	chat := &telebot.Chat{ID: chatID}
	topic := &telebot.Topic{ThreadID: int(topicID)}
	err := c.Bot.ReopenTopic(chat, topic)
	if err != nil {
		return fmt.Errorf("failed to reopen topic: %w", err)
	}
	return nil
}

// DeleteTopic deletes a topic from a supergroup
func (c *Client) DeleteTopic(chatID int64, topicID int64) error {
	chat := &telebot.Chat{ID: chatID}
	topic := &telebot.Topic{ThreadID: int(topicID)}
	err := c.Bot.DeleteTopic(chat, topic)
	if err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}
	return nil
}

// EditMessageText edits a text message
func (c *Client) EditMessageText(chat *telebot.Chat, messageID int, text string) error {
	msg := &telebot.Message{Chat: chat, ID: messageID}
	_, err := c.Bot.Edit(msg, text)
	if err != nil {
		return fmt.Errorf("failed to edit message text: %w", err)
	}
	return nil
}

// EditMessageCaption edits a message caption (for media)
func (c *Client) EditMessageCaption(chat *telebot.Chat, messageID int, caption string) error {
	msg := &telebot.Message{Chat: chat, ID: messageID, Caption: caption}
	_, err := c.Bot.Edit(msg, msg.Caption)
	if err != nil {
		return fmt.Errorf("failed to edit message caption: %w", err)
	}
	return nil
}

// DeleteMessage deletes a message
func (c *Client) DeleteMessage(chat *telebot.Chat, messageID int) error {
	msg := &telebot.Message{Chat: chat, ID: messageID}
	err := c.Bot.Delete(msg)
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
