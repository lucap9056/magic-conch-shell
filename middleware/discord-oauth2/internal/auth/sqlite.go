package auth

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type User struct {
	UserID   string
	Username string
}

type UserDevice struct {
	UserID     string
	DeviceID   string
	DeviceName string
	Secret     string
}

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`PRAGMA foreign_keys = ON;`)
	if err != nil {
		return nil, err
	}

	query := `
	CREATE TABLE IF NOT EXISTS users (
		user_id TEXT PRIMARY KEY,
		username TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS user_devices (
		device_id TEXT PRIMARY KEY DEFAULT (
			lower(hex(randomblob(4))) || '-' || 
			lower(hex(randomblob(2))) || '-4' || 
			substr(lower(hex(randomblob(2))),2) || '-' || 
			substr('89ab',abs(random()) % 4 + 1, 1) || 
			substr(lower(hex(randomblob(2))),2) || '-' || 
			lower(hex(randomblob(6)))
		),
		device_name TEXT NOT NULL,
		user_id TEXT NOT NULL,
		secret TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(user_id) ON DELETE CASCADE
	);`

	if _, err := db.Exec(query); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) SaveUser(userID, username string) error {
	query := `
	INSERT INTO users (user_id, username)
	VALUES (?, ?)
	ON CONFLICT(user_id) DO UPDATE SET 
		username = excluded.username,
	`
	_, err := d.db.Exec(query, userID, username)
	return err
}

func (d *Database) GetUser(userID string) (*User, error) {
	var user User
	query := `
	SELECT user_id, username
	FROM users
	WHERE user_id = ?;
	`
	err := d.db.QueryRow(query, userID).Scan(&user.UserID, &user.Username)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *Database) SaveDeviceSecret(userID, deviceName, secret string) (string, error) {
	query := `
	INSERT INTO user_devices (device_name, user_id, secret, updated_at)
	VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	RETURNING device_id;
	`
	var deviceID string
	err := d.db.QueryRow(query, deviceName, userID, secret).Scan(&deviceID)
	if err != nil {
		return "", err
	}
	return deviceID, nil
}

func (d *Database) UpdateDeviceSecret(deviceID, secret string) error {
	query := `
	UPDATE user_devices 
	SET secret = ?, updated_at = CURRENT_TIMESTAMP
	WHERE device_id = ?;
	`
	_, err := d.db.Exec(query, secret, deviceID)
	return err
}

func (d *Database) GetDeviceSecret(deviceID string) (string, error) {
	var secret string
	err := d.db.QueryRow("SELECT secret FROM user_devices WHERE device_id = ?", deviceID).Scan(&secret)
	if err != nil {
		return "", err
	}
	return secret, nil
}

func (d *Database) DeleteDevice(userID, deviceID string) error {
	query := `DELETE FROM user_devices WHERE user_id = ? AND device_id = ?`
	_, err := d.db.Exec(query, userID, deviceID)
	return err
}
