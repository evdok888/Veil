package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the database connection
type DB struct {
	conn *sql.DB
}

// Chat represents a chat mapping between client and topic
type Chat struct {
	UserID      int64
	TopicID     int64
	DisplayName string
	Blocked     bool
	CreatedAt   time.Time
}

// MessageLink represents a link between client and group messages
type MessageLink struct {
	ID              int64
	UserID          int64
	ClientMessageID int64
	GroupMessageID  int64
	TopicID         int64
	Direction       string // "client_to_owner" or "owner_to_client"
	CreatedAt       time.Time
}

// New creates a new database connection and runs migrations
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	_, err = conn.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := &DB{conn: conn}

	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate runs database migrations
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS chats (
		user_id INTEGER PRIMARY KEY,
		topic_id INTEGER NOT NULL UNIQUE,
		display_name TEXT,
		blocked BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS message_links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		client_message_id INTEGER NOT NULL,
		group_message_id INTEGER NOT NULL,
		topic_id INTEGER NOT NULL,
		direction TEXT CHECK(direction IN ('client_to_owner', 'owner_to_client')),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES chats(user_id)
	);

	CREATE INDEX IF NOT EXISTS idx_message_links_client ON message_links(user_id, client_message_id);
	CREATE INDEX IF NOT EXISTS idx_message_links_group ON message_links(topic_id, group_message_id);
	CREATE INDEX IF NOT EXISTS idx_chats_topic_id ON chats(topic_id);
	CREATE INDEX IF NOT EXISTS idx_chats_blocked ON chats(blocked);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// GetChatByUserID returns a chat by user ID
func (db *DB) GetChatByUserID(userID int64) (*Chat, error) {
	var chat Chat
	var createdAtStr string

	err := db.conn.QueryRow(`
		SELECT user_id, topic_id, display_name, blocked, created_at 
		FROM chats WHERE user_id = ?
	`, userID).Scan(&chat.UserID, &chat.TopicID, &chat.DisplayName, &chat.Blocked, &createdAtStr)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	chat.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	return &chat, nil
}

// GetChatByTopicID returns a chat by topic ID
func (db *DB) GetChatByTopicID(topicID int64) (*Chat, error) {
	var chat Chat
	var createdAtStr string

	err := db.conn.QueryRow(`
		SELECT user_id, topic_id, display_name, blocked, created_at 
		FROM chats WHERE topic_id = ?
	`, topicID).Scan(&chat.UserID, &chat.TopicID, &chat.DisplayName, &chat.Blocked, &createdAtStr)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	chat.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	return &chat, nil
}

// CreateChat creates a new chat mapping
func (db *DB) CreateChat(userID int64, topicID int64, displayName string) error {
	_, err := db.conn.Exec(`
		INSERT INTO chats (user_id, topic_id, display_name) VALUES (?, ?, ?)
	`, userID, topicID, displayName)
	return err
}

// SetBlocked sets the blocked status for a user
func (db *DB) SetBlocked(userID int64, blocked bool) error {
	_, err := db.conn.Exec(`UPDATE chats SET blocked = ? WHERE user_id = ?`, blocked, userID)
	return err
}

// GetMessageLinkByClientMessage returns a message link by client message ID
func (db *DB) GetMessageLinkByClientMessage(userID, clientMessageID int64) (*MessageLink, error) {
	var link MessageLink
	var createdAtStr string

	err := db.conn.QueryRow(`
		SELECT id, user_id, client_message_id, group_message_id, topic_id, direction, created_at
		FROM message_links 
		WHERE user_id = ? AND client_message_id = ?
	`, userID, clientMessageID).Scan(
		&link.ID, &link.UserID, &link.ClientMessageID, &link.GroupMessageID,
		&link.TopicID, &link.Direction, &createdAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	link.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	return &link, nil
}

// GetMessageLinkByGroupMessage returns a message link by group message ID
func (db *DB) GetMessageLinkByGroupMessage(topicID, groupMessageID int64) (*MessageLink, error) {
	var link MessageLink
	var createdAtStr string

	err := db.conn.QueryRow(`
		SELECT id, user_id, client_message_id, group_message_id, topic_id, direction, created_at
		FROM message_links 
		WHERE topic_id = ? AND group_message_id = ?
	`, topicID, groupMessageID).Scan(
		&link.ID, &link.UserID, &link.ClientMessageID, &link.GroupMessageID,
		&link.TopicID, &link.Direction, &createdAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	link.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	return &link, nil
}

// CreateMessageLink creates a new message link
func (db *DB) CreateMessageLink(userID, clientMessageID, groupMessageID, topicID int64, direction string) error {
	_, err := db.conn.Exec(`
		INSERT INTO message_links (user_id, client_message_id, group_message_id, topic_id, direction)
		VALUES (?, ?, ?, ?, ?)
	`, userID, clientMessageID, groupMessageID, topicID, direction)
	return err
}

// UpdateMessageLink updates an existing message link (for edited messages)
func (db *DB) UpdateMessageLink(userID, oldClientMessageID, newClientMessageID, newGroupMessageID int64) error {
	_, err := db.conn.Exec(`
		UPDATE message_links 
		SET client_message_id = ?, group_message_id = ?
		WHERE user_id = ? AND client_message_id = ?
	`, newClientMessageID, newGroupMessageID, userID, oldClientMessageID)
	return err
}

// DeleteMessageLink deletes a message link
func (db *DB) DeleteMessageLink(userID, clientMessageID int64) error {
	_, err := db.conn.Exec(`DELETE FROM message_links WHERE user_id = ? AND client_message_id = ?`, userID, clientMessageID)
	return err
}

// DeleteMessageLinksByTopic deletes all message links for a topic
func (db *DB) DeleteMessageLinksByTopic(topicID int64) error {
	_, err := db.conn.Exec(`DELETE FROM message_links WHERE topic_id = ?`, topicID)
	return err
}
