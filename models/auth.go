package models

import (
	"time"
)

type User struct {
	ID                 string    `json:"id"`
	Email              string    `json:"email"`
	PasswordHash       string    `json:"-"`
	SubscriptionTier   string    `json:"subscription_tier"`
	SubscriptionStatus string    `json:"subscription_status"`
	CreatedAt          time.Time `json:"created_at"`
}
