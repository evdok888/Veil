package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"gopkg.in/telebot.v3"

	"telegram-relay-bot/internal/config"
	"telegram-relay-bot/internal/db"
	"telegram-relay-bot/internal/telegram"
)

// CommandHandler handles admin commands from the owner
type CommandHandler struct {
	db       *db.DB
	tgClient *telegram.Client
	config   *config.Config
	logger   *log.Logger
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(db *db.DB, tgClient *telegram.Client, cfg *config.Config, logger *log.Logger) *CommandHandler {
	return &CommandHandler{
		db:       db,
		tgClient: tgClient,
		config:   cfg,
		logger:   logger,
	}
}

// HandleCommand processes commands in the supergroup
func (h *CommandHandler) HandleCommand(c telebot.Context) error {
	message := c.Message()

	// Verify this is from the owner in the supergroup
	if message.Chat.ID != h.config.SuperGroupID || message.Sender.ID != h.config.OwnerUserID {
		return nil
	}

	text := strings.TrimSpace(message.Text)
	if !strings.HasPrefix(text, "/") {
		return nil
	}

	parts := strings.Fields(text)
	command := strings.TrimPrefix(parts[0], "/")

	h.logger.Printf("[INFO] Received command: command=%s sender_id=%d", command, message.Sender.ID)

	switch command {
	case "end", "block":
		return h.handleBlock(c, message)
	case "unblock":
		return h.handleUnblock(c, message)
	case "del", "delete":
		return h.handleDelete(c, message)
	case "help":
		return h.handleHelp(c)
	default:
		return nil
	}
}

// HandleCallback processes inline button callbacks
func (h *CommandHandler) HandleCallback(c telebot.Context) error {
	callback := c.Callback()
	action, targetID := ExtractCallbackData(callback.Data)

	h.logger.Printf("[INFO] Received callback: action=%s target_id=%d", action, targetID)

	// Verify owner
	if callback.Sender.ID != h.config.OwnerUserID {
		h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
			CallbackID: callback.ID,
			Text:       "⛔️ Недоступно",
			ShowAlert:  true,
		})
		return nil
	}

	switch action {
	case "block":
		return h.handleBlockByCallback(c, targetID)
	case "delete":
		return h.handleDeleteByCallback(c, targetID)
	case "close_topic":
		return h.handleCloseTopic(c, targetID)
	default:
		h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
			CallbackID: callback.ID,
			Text:       "Неизвестное действие",
		})
	}

	return nil
}

func (h *CommandHandler) handleBlock(c telebot.Context, message *telebot.Message) error {
	var userID int64
	var topicID int64

	// Check if replying to a message in a topic
	if message.ThreadID != 0 {
		topicID = int64(message.ThreadID)
	} else {
		// Try to parse user ID from command args
		if len(strings.Fields(message.Text)) > 1 {
			idStr := strings.Fields(message.Text)[1]
			var err error
			userID, err = strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				_ = c.Send("❌ Неверный формат user_id")
				return nil
			}
		}
	}

	// If we have topic ID, look up user
	if topicID > 0 {
		chat, err := h.db.GetChatByTopicID(topicID)
		if err != nil || chat == nil {
			_ = c.Send("❌ Тема не найдена в базе данных")
			return nil
		}
		userID = chat.UserID
	}

	if userID == 0 {
		_ = c.Send("❌ Укажите user_id или ответьте командой на сообщение в теме")
		return nil
	}

	// Block the user
	err := h.db.SetBlocked(userID, true)
	if err != nil {
		_ = c.Send(fmt.Sprintf("❌ Ошибка блокировки: %v", err))
		return nil
	}

	// Close the topic
	chat, _ := h.db.GetChatByUserID(userID)
	if chat != nil {
		_ = h.tgClient.CloseTopic(h.config.SuperGroupID, chat.TopicID)
	}

	h.logger.Printf("[INFO] User blocked: user_id=%d", userID)
	_ = c.Send(fmt.Sprintf("✅ Пользователь %d заблокирован. Диалог завершён.", userID))

	return nil
}

func (h *CommandHandler) handleUnblock(c telebot.Context, message *telebot.Message) error {
	var userID int64

	// Parse user ID from command args
	fields := strings.Fields(message.Text)
	if len(fields) > 1 {
		var err error
		userID, err = strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			_ = c.Send("❌ Неверный формат user_id")
			return nil
		}
	}

	if userID == 0 {
		_ = c.Send("❌ Укажите user_id после команды /unblock")
		return nil
	}

	// Unblock the user
	err := h.db.SetBlocked(userID, false)
	if err != nil {
		_ = c.Send(fmt.Sprintf("❌ Ошибка разблокировки: %v", err))
		return nil
	}

	// Reopen the topic
	chat, _ := h.db.GetChatByUserID(userID)
	if chat != nil {
		_ = h.tgClient.ReopenTopic(h.config.SuperGroupID, chat.TopicID)
	}

	h.logger.Printf("[INFO] User unblocked: user_id=%d", userID)
	_ = c.Send(fmt.Sprintf("✅ Пользователь %d разблокирован.", userID))

	return nil
}

func (h *CommandHandler) handleDelete(c telebot.Context, message *telebot.Message) error {
	if message.ReplyTo == nil {
		_ = c.Send("❌ Ответьте командой /del на сообщение для удаления")
		return nil
	}

	// Get topic from the current message thread
	topicID := int64(message.ThreadID)

	if topicID == 0 {
		_ = c.Send("❌ Сообщение должно быть в теме")
		return nil
	}

	// Find message link
	link, err := h.db.GetMessageLinkByGroupMessage(topicID, int64(message.ReplyTo.ID))
	if err != nil || link == nil {
		_ = c.Send("❌ Сообщение не найдено в базе данных")
		return nil
	}

	// Delete from both sides
	groupChat := &telebot.Chat{ID: h.config.SuperGroupID}
	clientChat := &telebot.Chat{ID: link.UserID}

	// Delete in group (the original owner message)
	_ = h.tgClient.Delete(groupChat, int(link.GroupMessageID))

	// Delete on client side
	_ = h.tgClient.Delete(clientChat, int(link.ClientMessageID))

	// Remove from database
	_ = h.db.DeleteMessageLink(link.UserID, link.ClientMessageID)

	h.logger.Printf("[INFO] Message deleted, user_id=%d, message_id=%d", link.UserID, link.ClientMessageID)

	return nil
}

func (h *CommandHandler) handleBlockByCallback(c telebot.Context, userID int64) error {
	if userID == 0 {
		h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
			CallbackID: c.Callback().ID,
			Text:       "❌ Неверный user_id",
		})
		return nil
	}

	err := h.db.SetBlocked(userID, true)
	if err != nil {
		h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
			CallbackID: c.Callback().ID,
			Text:       fmt.Sprintf("❌ Ошибка: %v", err),
		})
		return nil
	}

	chat, _ := h.db.GetChatByUserID(userID)
	if chat != nil {
		_ = h.tgClient.CloseTopic(h.config.SuperGroupID, chat.TopicID)
	}

	h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
		CallbackID: c.Callback().ID,
		Text:       fmt.Sprintf("✅ Пользователь %d заблокирован", userID),
	})

	h.logger.Printf("[INFO] User blocked via callback, user_id=%d", userID)
	return nil
}

func (h *CommandHandler) handleDeleteByCallback(c telebot.Context, msgID int64) error {
	if msgID == 0 {
		h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
			CallbackID: c.Callback().ID,
			Text:       "❌ Неверный message_id",
		})
		return nil
	}

	// For callback deletion, we need to find the message by group message ID
	// This requires knowing the topic ID, which we can get from the message thread
	callback := c.Callback()
	topicID := int64(callback.Message.ThreadID)

	link, err := h.db.GetMessageLinkByGroupMessage(topicID, msgID)
	if err != nil || link == nil {
		h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
			CallbackID: c.Callback().ID,
			Text:       "❌ Сообщение не найдено",
		})
		return nil
	}

	groupChat := &telebot.Chat{ID: h.config.SuperGroupID}
	clientChat := &telebot.Chat{ID: link.UserID}

	_ = h.tgClient.Delete(groupChat, int(link.GroupMessageID))
	_ = h.tgClient.Delete(clientChat, int(link.ClientMessageID))
	_ = h.db.DeleteMessageLink(link.UserID, link.ClientMessageID)

	h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
		CallbackID: c.Callback().ID,
		Text:       "🗑 Удалено",
	})

	h.logger.Printf("[INFO] Message deleted via callback, user_id=%d", link.UserID)
	return nil
}

func (h *CommandHandler) handleCloseTopic(c telebot.Context, topicID int64) error {
	if topicID == 0 {
		h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
			CallbackID: c.Callback().ID,
			Text:       "❌ Неверный topic_id",
		})
		return nil
	}

	err := h.tgClient.CloseTopic(h.config.SuperGroupID, topicID)
	if err != nil {
		h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
			CallbackID: c.Callback().ID,
			Text:       fmt.Sprintf("❌ Ошибка: %v", err),
		})
		return nil
	}

	h.tgClient.Respond(c.Callback(), &telebot.CallbackResponse{
		CallbackID: c.Callback().ID,
		Text:       "✅ Тема закрыта",
	})

	h.logger.Printf("[INFO] Topic closed via callback, topic_id=%d", topicID)
	return nil
}

func (h *CommandHandler) handleHelp(c telebot.Context) error {
	helpText := `📋 **Команды бота:**

/block или /end - Заблокировать клиента и закрыть тему
  (ответьте на сообщение в теме или укажите user_id)

/unblock <user_id> - Разблокировать клиента и открыть тему

/del - Удалить сообщение
  (ответьте на сообщение в теме)

/help - Показать эту справку

**Кнопки под сообщениями:**
🚫 Завершить диалог - Блокирует клиента
🗑 Удалить - Удаляет сообщение с обеих сторон`

	_ = c.Send(helpText, &telebot.SendOptions{
		ParseMode: telebot.ModeMarkdown,
	})

	return nil
}

// CreateInlineKeyboardForMessage creates inline keyboard for messages
func (h *CommandHandler) CreateInlineKeyboardForMessage(userID int64, groupMsgID int64) *telebot.ReplyMarkup {
	return h.tgClient.NewReplyMarkup(
		telebot.Row{
			telebot.Btn{
				Unique: "block",
				Text:   "🚫 Завершить диалог",
				Data:   fmt.Sprintf("block:%d", userID),
			},
		},
		telebot.Row{
			telebot.Btn{
				Unique: "delete",
				Text:   "🗑 Удалить",
				Data:   fmt.Sprintf("delete:%d", groupMsgID),
			},
		},
	)
}
