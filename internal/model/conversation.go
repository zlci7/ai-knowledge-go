package model

import "time"

type Conversation struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"-"`
	ConvID    string    `gorm:"index;not null" json:"conv_id"`
	UserID    uint64    `gorm:"index;not null" json:"user_id"`
	Title     string    `gorm:"size:200" json:"title"`
	Summary   string    `gorm:"size:300" json:"summary"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsDeleted bool      `gorm:"default:false" json:"-"`
}
