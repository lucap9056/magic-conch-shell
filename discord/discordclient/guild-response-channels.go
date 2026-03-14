package discordclient

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// GuildResponseChannels manages the storage and retrieval of response channel IDs for Discord guilds.
type GuildResponseChannels struct {
	mu       sync.RWMutex
	channels map[string]string // In-memory cache of guild ID to channel ID mappings
	db       *sql.DB           // Database connection for persistence
}

func getDatabasePath() string {
	databaseFileName := "channels.db"

	executablePath, err := os.Executable()
	if err != nil {
		return filepath.Join(".", databaseFileName)
	}

	executableDir := filepath.Dir(executablePath)

	databaseFilePath := filepath.Join(executableDir, databaseFileName)

	return databaseFilePath
}

// NewGuildResponseChannels creates a new instance of GuildResponseChannels and initializes the database.
func newGuildResponseChannels() (*GuildResponseChannels, error) {

	databasePath := getDatabasePath()
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// SQL statement to create the guild_channels table if it doesn't exist.
	// It stores guild_id as PRIMARY KEY and channel_id.
	const createTableSQL = `
	CREATE TABLE IF NOT EXISTS guild_channels (
		guild_id TEXT NOT NULL PRIMARY KEY,
		channel_id TEXT NOT NULL
	);`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to create guild channels table: %w", err)
	}

	grc := &GuildResponseChannels{
		channels: make(map[string]string),
		db:       db,
	}

	// Load existing channel configurations from the database into memory.
	err = grc.loadChannelsFromDB()
	if err != nil {
		return nil, fmt.Errorf("failed to load channels from database: %w", err)
	}
	return grc, nil
}

// SetResponseChannel sets the response channel for a given guild.
// It updates both the in-memory cache and the persistent database.
func (grc *GuildResponseChannels) SetResponseChannel(guildID, channelID string) error {
	grc.mu.Lock()
	defer grc.mu.Unlock()

	stmt, err := grc.db.Prepare("INSERT OR REPLACE INTO guild_channels(guild_id, channel_id) VALUES(?, ?)")
	if err != nil {
		return fmt.Errorf("ERROR: Failed to prepare SQL statement for setting response channel: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(guildID, channelID)
	if err != nil {
		return fmt.Errorf("ERROR: Failed to save response channel for guild %s to database: %v", guildID, err)
	}

	grc.channels[guildID] = channelID
	return nil
}

// GetResponseChannel retrieves the response channel ID for a given guild ID from the in-memory cache.
// It returns an empty string if no channel is set for the guild.
func (grc *GuildResponseChannels) GetResponseChannel(guildID string) (channelID string, found bool) {
	grc.mu.RLock()
	defer grc.mu.RUnlock()

	channelID, found = grc.channels[guildID]
	return
}

// loadChannelsFromDB loads all guild channel mappings from the database into the in-memory cache.
func (grc *GuildResponseChannels) loadChannelsFromDB() error {
	rows, err := grc.db.Query("SELECT guild_id, channel_id FROM guild_channels")
	if err != nil {
		return fmt.Errorf("failed to query database for guild channels: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var guildID, channelID string
		if err := rows.Scan(&guildID, &channelID); err != nil {
			return fmt.Errorf("failed to scan row from database: %w", err)
		}
		grc.channels[guildID] = channelID
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error during rows iteration: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (grc *GuildResponseChannels) Close() error {
	if grc.db != nil {
		return grc.db.Close()
	}
	return nil
}
