package models

import "time"

type Surl struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	ShortURL  string    `json:"short_url"`
	Clicked   uint64    `json:"clicked"`
	IsAlias   bool      `json:"is_alias"`
	UpdatedAt time.Time `json:"updated_at"`
}

type User struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Password  string `json:"-"`
	IsEnabled bool   `json:"is_enabled"`
}
