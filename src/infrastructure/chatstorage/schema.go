package chatstorage

import "fmt"

// InitializeSchema creates or migrates the database schema
func (r *SQLiteRepository) InitializeSchema() error {
	// Get current schema version
	version, err := r.getSchemaVersion()
	if err != nil {
		return err
	}

	// Run migrations based on version
	migrations := r.getMigrations()
	for i := version; i < len(migrations); i++ {
		if err := r.runMigration(migrations[i], i+1); err != nil {
			return fmt.Errorf("failed to run migration %d: %w", i+1, err)
		}
	}

	return nil
}

// getSchemaVersion returns the current schema version
func (r *SQLiteRepository) getSchemaVersion() (int, error) {
	// Create schema_info table if it doesn't exist
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_info (
			version INTEGER PRIMARY KEY DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return 0, err
	}

	// Get current version
	var version int
	err = r.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_info").Scan(&version)
	if err != nil {
		return 0, err
	}

	return version, nil
}

// runMigration executes a migration
func (r *SQLiteRepository) runMigration(migration string, version int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Execute migration
	if _, err := tx.Exec(migration); err != nil {
		return err
	}

	// Update schema version
	if _, err := tx.Exec("INSERT OR REPLACE INTO schema_info (version) VALUES (?)", version); err != nil {
		return err
	}

	return tx.Commit()
}

// getMigrations returns all database migrations
func (r *SQLiteRepository) getMigrations() []string {
	return []string{
		// Migration 1: Initial schema with only chats and messages tables
		`
		-- Create chats table
		CREATE TABLE IF NOT EXISTS chats (
			jid TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			last_message_time TIMESTAMP NOT NULL,
			ephemeral_expiration INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		-- Create messages table
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT NOT NULL,
			chat_jid TEXT NOT NULL,
			sender TEXT NOT NULL,
			content TEXT,
			timestamp TIMESTAMP NOT NULL,
			is_from_me BOOLEAN DEFAULT FALSE,
			media_type TEXT,
			filename TEXT,
			url TEXT,
			media_key BLOB,
			file_sha256 BLOB,
			file_enc_sha256 BLOB,
			file_length INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, chat_jid),
			FOREIGN KEY (chat_jid) REFERENCES chats(jid) ON DELETE CASCADE
		);

		-- Create indexes for performance
		CREATE INDEX IF NOT EXISTS idx_messages_chat_jid ON messages(chat_jid);
		CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
		CREATE INDEX IF NOT EXISTS idx_messages_media_type ON messages(media_type);
		CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender);
		CREATE INDEX IF NOT EXISTS idx_chats_last_message ON chats(last_message_time);
		CREATE INDEX IF NOT EXISTS idx_chats_name ON chats(name);
		`,

		// Migration 2: Add index for message ID lookups (performance optimization)
		`
		CREATE INDEX IF NOT EXISTS idx_messages_id ON messages(id);
		`,

		// Migration 3: Add message status tracking with timestamps
		`
		-- Add status column (sent, delivered, read, played)
		ALTER TABLE messages ADD COLUMN status TEXT DEFAULT 'sent';

		-- Add timestamp columns for status tracking
		ALTER TABLE messages ADD COLUMN delivered_at TIMESTAMP;
		ALTER TABLE messages ADD COLUMN read_at TIMESTAMP;
		ALTER TABLE messages ADD COLUMN played_at TIMESTAMP;

		-- Create index for status queries
		CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
		`,

		// Migration 4: Add message_reactions table for storing emoji reactions
		`
		-- Create message_reactions table
		-- Each sender can have one reaction per message (upsert on conflict)
		CREATE TABLE IF NOT EXISTS message_reactions (
			message_id TEXT NOT NULL,
			chat_jid TEXT NOT NULL,
			sender_jid TEXT NOT NULL,
			emoji TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			PRIMARY KEY (message_id, chat_jid, sender_jid)
		);

		-- Create indexes for efficient lookups
		CREATE INDEX IF NOT EXISTS idx_reactions_message ON message_reactions(message_id, chat_jid);
		CREATE INDEX IF NOT EXISTS idx_reactions_chat ON message_reactions(chat_jid);
		`,
	}
}
