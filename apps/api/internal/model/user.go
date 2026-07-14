package model

import "time"

type User struct {
	Base
	Email        string     `json:"email"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	DisplayName  string     `json:"display_name"`
	AvatarURL    string     `json:"avatar_url"`
	Bio          string     `json:"bio"`
	Status       string     `json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

func (User) TableName() string {
	return "users"
}
