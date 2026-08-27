package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/telebot.v3"

	"telegram-relay-bot/internal/config"
	"telegram-relay-bot/internal/db"
	"telegram-relay-bot/internal/handlers"
	"telegram-relay-bot/internal/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := log.New(os.Stdout, "[BOT] ", log.LstdFlags|log.Lmicroseconds)

	logger.Printf("[INFO] Starting Telegram Relay Bot version=1.0.0 webhook_mode=%v", cfg.WebhookURL != "")

	database, err := db.New(cfg.DBPath)
	if err != nil {
		logger.Printf("[ERROR] Failed to initialize database: error=%v", err)
		os.Exit(1)
	}
	defer database.Close()
	logger.Printf("[INFO] Database initialized: path=%s", cfg.DBPath)

	settings := telebot.Settings{
		Token:     cfg.BotToken,
		ParseMode: telebot.ModeMarkdown,
	}

	var poller telebot.Poller
	if cfg.WebhookURL != "" {
		// Webhook mode: create a Webhook poller
		webhook := &telebot.Webhook{
			Listen:         cfg.ServerAddr,
			MaxConnections: 100,
		}

		// Set the webhook URL that Telegram will call
		// The Listen field is where our server listens locally
		// Telegram will send updates to cfg.WebhookURL (public HTTPS URL)
		poller = webhook
		logger.Printf("[INFO] Webhook mode enabled: url=%s listen=%s", cfg.WebhookURL, cfg.ServerAddr)
	} else {
		// Long polling mode for local development
		poller = &telebot.LongPoller{Timeout: 10 * time.Second}
		logger.Printf("[INFO] Polling mode enabled")
	}

	settings.Poller = poller

	tgClient, err := telegram.NewClient(cfg.BotToken, settings)
	if err != nil {
		logger.Printf("[ERROR] Failed to create Telegram client: error=%v", err)
		os.Exit(1)
	}

	clientHandler := handlers.NewClientHandler(database, tgClient, cfg, logger)
	ownerHandler := handlers.NewOwnerHandler(database, tgClient, cfg, logger)
	commandHandler := handlers.NewCommandHandler(database, tgClient, cfg, logger)

	tgClient.Handle(telebot.OnText, func(c telebot.Context) error {
		if c.Chat().Type == telebot.ChatPrivate {
			return clientHandler.HandleMessage(c)
		}
		return ownerHandler.HandleMessage(c)
	})

	tgClient.Handle(telebot.OnPhoto, func(c telebot.Context) error {
		if c.Chat().Type == telebot.ChatPrivate {
			return clientHandler.HandleMessage(c)
		}
		return ownerHandler.HandleMessage(c)
	})

	tgClient.Handle(telebot.OnVoice, func(c telebot.Context) error {
		if c.Chat().Type == telebot.ChatPrivate {
			return clientHandler.HandleMessage(c)
		}
		return ownerHandler.HandleMessage(c)
	})

	tgClient.Handle(telebot.OnVideo, func(c telebot.Context) error {
		if c.Chat().Type == telebot.ChatPrivate {
			return clientHandler.HandleMessage(c)
		}
		return ownerHandler.HandleMessage(c)
	})

	tgClient.Handle(telebot.OnAudio, func(c telebot.Context) error {
		if c.Chat().Type == telebot.ChatPrivate {
			return clientHandler.HandleMessage(c)
		}
		return ownerHandler.HandleMessage(c)
	})

	tgClient.Handle(telebot.OnDocument, func(c telebot.Context) error {
		if c.Chat().Type == telebot.ChatPrivate {
			return clientHandler.HandleMessage(c)
		}
		return ownerHandler.HandleMessage(c)
	})

	tgClient.Handle(telebot.OnSticker, func(c telebot.Context) error {
		if c.Chat().Type == telebot.ChatPrivate {
			return clientHandler.HandleMessage(c)
		}
		return ownerHandler.HandleMessage(c)
	})

	tgClient.Handle(telebot.OnAnimation, func(c telebot.Context) error {
		if c.Chat().Type == telebot.ChatPrivate {
			return clientHandler.HandleMessage(c)
		}
		return ownerHandler.HandleMessage(c)
	})

	tgClient.Handle(telebot.OnEdited, func(c telebot.Context) error {
		if c.Chat().Type == telebot.ChatPrivate {
			return clientHandler.HandleEditedMessage(c)
		}
		return ownerHandler.HandleEditedMessage(c)
	})

	tgClient.Handle("/end", commandHandler.HandleCommand)
	tgClient.Handle("/block", commandHandler.HandleCommand)
	tgClient.Handle("/unblock", commandHandler.HandleCommand)
	tgClient.Handle("/del", commandHandler.HandleCommand)
	tgClient.Handle("/delete", commandHandler.HandleCommand)
	tgClient.Handle("/help", commandHandler.HandleCommand)

	tgClient.Handle(telebot.OnCallback, commandHandler.HandleCallback)

	// Set up error handler using Settings.OnError instead of bot.Error
	settings.OnError = func(err error, c telebot.Context) {
		logger.Printf("[ERROR] Telegram bot error: error=%v", err)
	}

	// Recreate bot with error handler since OnError must be set before Start
	tgClient, err = telegram.NewClient(cfg.BotToken, settings)
	if err != nil {
		logger.Printf("[ERROR] Failed to recreate Telegram client with error handler: error=%v", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Printf("[INFO] Shutting down...")
		cancel()
	}()

	logger.Printf("[INFO] Bot is starting...")

	done := make(chan bool)
	go func() {
		tgClient.Start()
		done <- true
	}()

	select {
	case <-ctx.Done():
		logger.Printf("[INFO] Graceful shutdown initiated")
		time.Sleep(2 * time.Second)
	case <-done:
		logger.Printf("[INFO] Bot stopped")
	}

	logger.Printf("[INFO] Bot shutdown complete")
}
