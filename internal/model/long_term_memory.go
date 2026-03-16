package model

import "time"

type MemoryCategory string

const (
	MemoryCategoryPreference MemoryCategory = "preference"
	MemoryCategoryFact       MemoryCategory = "fact"
	MemoryCategoryRule       MemoryCategory = "rule"
)

type MemorySource string

const (
	MemorySourceManual    MemorySource = "manual"
	MemorySourceExtracted MemorySource = "extracted"
)

type MemoryVectorStatus string

const (
	MemoryVectorStatusPending  MemoryVectorStatus = "pending"
	MemoryVectorStatusSynced   MemoryVectorStatus = "synced"
	MemoryVectorStatusFailed   MemoryVectorStatus = "failed"
	MemoryVectorStatusDeleting MemoryVectorStatus = "deleting"
	MemoryVectorStatusDeleted  MemoryVectorStatus = "deleted"
)

type LongTermMemory struct {
	ID               uint64             `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           uint64             `gorm:"not null;index:idx_user;index:idx_category,priority:1" json:"user_id"`
	Content          string             `gorm:"type:text;not null" json:"content"`
	Category         MemoryCategory     `gorm:"size:50;index:idx_category,priority:2" json:"category"`
	VectorID         string             `gorm:"size:100" json:"vector_id,omitempty"`
	VectorStatus     MemoryVectorStatus `gorm:"size:20;not null;default:pending" json:"-"`
	VectorRetryCount int                `gorm:"not null;default:0" json:"-"`
	VectorLastError  string             `gorm:"type:text" json:"-"`
	Source           MemorySource       `gorm:"size:20;not null;default:manual" json:"source"`
	CreatedAt        time.Time          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time          `gorm:"autoUpdateTime" json:"updated_at"`
	IsDeleted        bool               `gorm:"not null;default:false" json:"is_deleted"`
}

func (LongTermMemory) TableName() string {
	return "long_term_memories"
}
