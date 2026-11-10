package handlers

import (
	"fmt"
	"log"
	"strings"
	"strconv"

	"gopkg.in/telebot.v3"

	"telegram-relay-bot/internal/config"
	"telegram-relay-bot/internal/db"
	"telegram-relay-bot/internal/telegram"
)

// OwnerHandler handles messages from the owner in the supergroup
type OwnerHandler struct {
	db       *db.DB
	tgClient *telegram.Client
	config   *config.Config
	logger   *log.Logger
}

// NewOwnerHandler creates a new owner message handler
func NewOwnerHandler(db *db.DB, tgClient *telegram.Client, cfg *config.Config, logger *log.Logger) *OwnerHandler {
	return &OwnerHandler{
		db:       db,
		tgClient: tgClient,
		config:   cfg,
		logger:   logger,
	}
}

// HandleMessage processes messages from the owner in topics
func (h *OwnerHandler) HandleMessage(c telebot.Context) error {
	message := c.Message()

	// Verify this is from the owner in the supergroup
	if !h.isValidOwnerMessage(c) {
		return nil
	}

	// Get topic ID from message thread
	topicID := int64(message.ThreadID)
	if topicID == 0 {
		// Message not in a topic, ignore
		return nil
	}

	h.logger.Printf("[INFO] Received message from owner: topic_id=%d message_id=%d content_type=%s",
		topicID, message.ID, getContentType(message))

	// Find the client for this topic
	chat, err := h.db.GetChatByTopicID(topicID)
	if err != nil || chat == nil {
		h.logger.Printf("[ERROR] No chat found for topic: topic_id=%d error=%v", topicID, err)
		return nil
	}

	if chat.Blocked {
		h.logger.Printf("[WARN] Trying to send to blocked user: user_id=%d", chat.UserID)
		return nil
	}

	// Copy message to the client
	clientChat := &telebot.Chat{ID: chat.UserID}
	copiedMsg, err := h.tgClient.CopyMessageToUser(clientChat, message)
	if err != nil {
		h.logger.Printf("[ERROR] Failed to copy message to client: user_id=%d error=%v", chat.UserID, err)
		return nil
	}

	// Save message link
	err = h.db.CreateMessageLink(
		chat.UserID,
		int64(copiedMsg.ID),
		int64(message.ID),
		topicID,
		"owner_to_client",
	)
	if err != nil {
		h.logger.Printf("[ERROR] Failed to save message link, user_id=%d, error=%v", chat.UserID, err)
	}

	h.logger.Printf("[INFO] Message copied to client, user_id=%d, client_message_id=%d", chat.UserID, copiedMsg.ID)

	return nil
}

// HandleEditedMessage processes edited messages from the owner
func (h *OwnerHandler) HandleEditedMessage(c telebot.Context) error {
	message := c.Message()

	// Verify this is from the owner in the supergroup
	if !h.isValidOwnerMessage(c) {
		return nil
	}

	// Get topic ID from message thread
	topicID := int64(message.ThreadID)
	if topicID == 0 {
		return nil
	}

	h.logger.Printf("[INFO] Received edited message from owner: topic_id=%d message_id=%d",
		topicID, message.ID)

	// Find the message link
	link, err := h.db.GetMessageLinkByGroupMessage(topicID, int64(message.ID))
	if err != nil || link == nil {
		h.logger.Printf("[WARN] No message link found for edited message: topic_id=%d message_id=%d", topicID, message.ID)
		return nil
	}

	// Edit the corresponding message on the client side
	clientChat := &telebot.Chat{ID: link.UserID}

	if message.Text != "" {
		err = h.tgClient.EditMessageText(clientChat, int(link.ClientMessageID), message.Text)
	} else if message.Caption != "" {
		err = h.tgClient.EditMessageCaption(clientChat, int(link.ClientMessageID), message.Caption)
	} else {
		h.logger.Printf("[INFO] Cannot edit media content, only captions: topic_id=%d", topicID)
		return nil
	}

	if err != nil {
		h.logger.Printf("[ERROR] Failed to edit message on client side: user_id=%d error=%v", link.UserID, err)
		return nil
	}

	h.logger.Printf("[INFO] Message edited on client side: user_id=%d client_message_id=%d",
		link.UserID, link.ClientMessageID)

	return nil
}

func (h *OwnerHandler) isValidOwnerMessage(c telebot.Context) bool {
	message := c.Message()

	// Check if message is from the supergroup
	if message.Chat.ID != h.config.SuperGroupID {
		return false
	}

	// Check if message is from the owner
	if message.Sender.ID != h.config.OwnerUserID {
		h.logger.Printf("[WARN] Message from non-owner in supergroup, sender_id=%d, expected_owner=%d", message.Sender.ID, h.config.OwnerUserID)
		return false
	}

	return true
}

// Helper to extract inline query data for callback handling
func ExtractCallbackData(callback string) (action string, targetID int64) {
	action, rest, found := strings.Cut(callback, ":")
	if found {
		if n, err := strconv.ParseInt(rest, 10, 64); err == nil {
			targetID = n
		}
	}
	return action, targetID
}