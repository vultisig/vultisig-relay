package model

import (
	"database/sql"
	"time"
)

type User struct {
	ID         int64        `json:"id,omitempty"`
	APIKey     string       `json:"api_key,omitempty"`
	CreatedAt  sql.NullTime `json:"created_at"`
	ExpiredAt  sql.NullTime `json:"expired_at"`
	NoOfVaults int64        `json:"no_of_vaults,omitempty"`
	IsPaid     bool         `json:"is_paid,omitempty"`
}

func (u User) IsValid() bool {
	if u.ID == 0 ||
		u.APIKey == "" ||
		!u.CreatedAt.Valid {
		return false
	}
	// user didn't paid
	if !u.IsPaid {
		return false
	}
	// user expired
	if u.ExpiredAt.Valid && u.ExpiredAt.Time.Before(time.Now()) {
		return false
	}
	if u.NoOfVaults == 0 {
		return false
	}
	return true
}
