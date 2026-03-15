package model

import "gorm.io/gorm"

// Migrate 自动迁移数据库结构
func Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&User{},
		&Conversation{},
		&Message{},
	)
	return err
}
