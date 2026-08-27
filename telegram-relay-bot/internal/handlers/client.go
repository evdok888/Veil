package handlers

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"gopkg.in/telebot.v3"

	"telegram-relay-bot/internal/config"
	"telegram-relay-bot/internal/db"
	"telegram-relay-bot/internal/telegram"
)

// ClientHandler handles messages from clients
type ClientHandler struct {
	db           *db.DB
	tgClient     *telegram.Client
	config       *config.Config
	rateLimiters map[int64]*rateLimiter
	mu           sync.RWMutex
	logger       *log.Logger
}

type rateLimiter struct {
	messages   []time.Time
	mu         sync.Mutex
	limit      int
	windowSize time.Duration
}

func newRateLimiter(limit int, windowSize time.Duration) *rateLimiter {
	return &rateLimiter{
		messages:   make([]time.Time, 0),
		limit:      limit,
		windowSize: windowSize,
	}
}

func (rl *rateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.windowSize)

	validMessages := make([]time.Time, 0)
	for _, t := range rl.messages {
		if t.After(windowStart) {
			validMessages = append(validMessages, t)
		}
	}
	rl.messages = validMessages

	if len(rl.messages) >= rl.limit {
		return false
	}

	rl.messages = append(rl.messages, now)
	return true
}

func NewClientHandler(db *db.DB, tgClient *telegram.Client, cfg *config.Config, logger *log.Logger) *ClientHandler {
	return &ClientHandler{
		db:           db,
		tgClient:     tgClient,
		config:       cfg,
		rateLimiters: make(map[int64]*rateLimiter),
		logger:       logger,
	}
}

func (h *ClientHandler) HandleMessage(c telebot.Context) error {
	userID := c.Sender().ID
	message := c.Message()

	h.logger.Printf("[INFO] Received message from client: user_id=%d message_id=%d content_type=%s",
		userID, message.ID, getContentType(message))

	if h.config.RateLimitPerMinute > 0 {
		if !h.checkRateLimit(userID) {
			h.logger.Printf("[WARN] Rate limit exceeded: user_id=%d", userID)
			return nil
		}
	}

	chat, err := h.db.GetChatByUserID(userID)
	if err != nil {
		h.logger.Printf("[ERROR] Failed to get chat: user_id=%d error=%v", userID, err)
		return nil
	}

	if chat != nil && chat.Blocked {
		h.logger.Printf("[INFO] Blocked user tried to send message: user_id=%d", userID)
		_, _ = h.tgClient.Send(c.Chat(), h.config.BlockedMessage)
		return nil
	}

	topicID, err := h.getOrCreateTopic(c.Sender(), userID)
	if err != nil {
		h.logger.Printf("[ERROR] Failed to get/create topic: user_id=%d error=%v", userID, err)
		return nil
	}

	copiedMsg, err := h.tgClient.CopyMessageToTopic(
		h.config.SuperGroupID,
		topicID,
		message.Chat.ID,
		message.ID,
	)
	if err != nil {
		h.logger.Printf("[ERROR] Failed to copy message to group: user_id=%d error=%v", userID, err)
		return nil
	}

	err = h.db.CreateMessageLink(
		userID,
		int64(message.ID),
		int64(copiedMsg.ID),
		topicID,
		"client_to_owner",
	)
	if err != nil {
		h.logger.Printf("[ERROR] Failed to save message link: user_id=%d error=%v", userID, err)
	}

	h.logger.Printf("[INFO] Message copied to topic: user_id=%d topic_id=%d group_message_id=%d",
		userID, topicID, copiedMsg.ID)

	return nil
}

func (h *ClientHandler) HandleEditedMessage(c telebot.Context) error {
	userID := c.Sender().ID
	message := c.Message()

	h.logger.Printf("[INFO] Received edited message from client: user_id=%d message_id=%d",
		userID, message.ID)

	link, err := h.db.GetMessageLinkByClientMessage(userID, int64(message.ID))
	if err != nil || link == nil {
		h.logger.Printf("[WARN] No message link found for edited message: user_id=%d message_id=%d", userID, message.ID)
		return nil
	}

	groupChat := &telebot.ChatID{ID: h.config.SuperGroupID}

	if message.Text != "" {
		err = h.tgClient.EditMessageText(groupChat, int(link.GroupMessageID), message.Text)
	} else if message.Caption != "" {
		err = h.tgClient.EditMessageCaption(groupChat, int(link.GroupMessageID), message.Caption)
	} else {
		h.logger.Printf("[INFO] Cannot edit media content, only captions: user_id=%d", userID)
		return nil
	}

	if err != nil {
		h.logger.Printf("[ERROR] Failed to edit message in group: user_id=%d error=%v", userID, err)
		return nil
	}

	h.logger.Printf("[INFO] Message edited in group: user_id=%d group_message_id=%d",
		userID, link.GroupMessageID)

	return nil
}

func (h *ClientHandler) getOrCreateTopic(user *telebot.User, userID int64) (int64, error) {
	chat, err := h.db.GetChatByUserID(userID)
	if err != nil {
		return 0, err
	}

	if chat != nil {
		return chat.TopicID, nil
	}

	displayName := fmt.Sprintf("%s (%d)", getUserName(user), userID)
	topicID, err := h.tgClient.CreateTopic(h.config.SuperGroupID, displayName)
	if err != nil {
		return 0, fmt.Errorf("failed to create topic: %w", err)
	}

	err = h.db.CreateChat(userID, topicID, displayName)
	if err != nil {
		h.logger.Printf("[ERROR] Failed to save chat mapping: user_id=%d error=%v", userID, err)
	}

	h.logger.Printf("[INFO] Created new topic: user_id=%d topic_id=%d display_name=%s", userID, topicID, displayName)

	return topicID, nil
}

func (h *ClientHandler) checkRateLimit(userID int64) bool {
	h.mu.RLock()
	limiter, exists := h.rateLimiters[userID]
	h.mu.RUnlock()

	if !exists {
		h.mu.Lock()
		if limiter, exists = h.rateLimiters[userID]; !exists {
			limiter = newRateLimiter(h.config.RateLimitPerMinute, time.Minute)
			h.rateLimiters[userID] = limiter
		}
		h.mu.Unlock()
	}

	return limiter.Allow()
}

func getUserName(user *telebot.User) string {
	if user.FirstName != "" && user.LastName != "" {
		return strings.TrimSpace(user.FirstName + " " + user.LastName)
	}
	if user.FirstName != "" {
		return user.FirstName
	}
	if user.Username != "" {
		return "@" + user.Username
	}
	return "Unknown"
}

func getContentType(msg *telebot.Message) string {
	switch {
	case msg.Text != "":
		return "text"
	case msg.Photo != nil && len(msg.Photo) > 0:
		return "photo"
	case msg.Voice != nil:
		return "voice"
	case msg.Video != nil:
		return "video"
	case msg.Audio != nil:
		return "audio"
	case msg.Document != nil:
		return "document"
	case msg.Sticker != nil:
		return "sticker"
	case msg.Animation != nil:
		return "animation"
	default:
		return "unknown"
	}
}
