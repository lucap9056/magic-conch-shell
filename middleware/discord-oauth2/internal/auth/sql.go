package auth

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type User struct {
	UserID   string
	Username string
	Email    string
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

func NewDatabase(dsn string) (*Database, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(20)

	db.SetMaxIdleConns(15)

	db.SetConnMaxLifetime(5 * time.Minute)

	db.SetConnMaxIdleTime(2 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	query := `
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

	CREATE TABLE IF NOT EXISTS users (
		user_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		username TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE
	);

	CREATE TABLE IF NOT EXISTS user_devices (
		device_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		device_name TEXT NOT NULL,
		user_id UUID NOT NULL,
		secret TEXT NOT NULL,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(user_id) ON DELETE CASCADE
	);`

	if _, err := db.Exec(query); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) GetUserFromEmail(email string) (*User, error) {
	var user User
	query := `
	SELECT user_id, username, email
	FROM users
	WHERE email = $1;
	`
	err := d.db.QueryRow(query, email).Scan(&user.UserID, &user.Username, &user.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (d *Database) GetUserFromID(userID string) (*User, error) {
	var user User
	query := `
	SELECT user_id, username, email
	FROM users
	WHERE user_id = $1;
	`
	err := d.db.QueryRow(query, userID).Scan(&user.UserID, &user.Username, &user.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (d *Database) SaveDeviceSecret(userID, deviceName, secret string) (string, error) {
	query := `
	INSERT INTO user_devices (device_name, user_id, secret, updated_at)
	VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
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
	SET secret = $1, updated_at = CURRENT_TIMESTAMP
	WHERE device_id = $2;
	`
	_, err := d.db.Exec(query, secret, deviceID)
	return err
}

func (d *Database) GetDeviceSecret(deviceID string) (string, error) {
	var secret string
	err := d.db.QueryRow("SELECT secret FROM user_devices WHERE device_id = $1", deviceID).Scan(&secret)
	if err != nil {
		return "", err
	}
	return secret, nil
}

func (d *Database) DeleteDevice(userID, deviceID string) error {
	query := `DELETE FROM user_devices WHERE user_id = $1 AND device_id = $2`
	_, err := d.db.Exec(query, userID, deviceID)
	return err
}
