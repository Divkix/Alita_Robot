package db

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetAdminSettings Get admin settings for a chat
func GetAdminSettings(chatID int64) *AdminSettings {
	return checkAdminSetting(chatID)
}

// checkAdminSetting retrieves or creates default admin settings for a chat.
// It returns default settings if the record is not found or an error occurs.
func checkAdminSetting(chatID int64) (adminSrc *AdminSettings) {
	adminSrc = &AdminSettings{}

	err := GetRecord(adminSrc, AdminSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create default settings
		adminSrc = &AdminSettings{ChatId: chatID, AnonAdmin: false}
		err := CreateRecord(adminSrc)
		if err != nil {
			log.Errorf("[Database][checkAdminSetting]: %v ", err)
		}
	} else if err != nil {
		// Return default on error
		adminSrc = &AdminSettings{ChatId: chatID, AnonAdmin: false}
		log.Errorf("[Database][checkAdminSetting]: %v ", err)
	}
	return adminSrc
}

// SetAnonAdminMode Set anon admin mode for a chat
func SetAnonAdminMode(chatID int64, val bool) error {
	adminSrc := checkAdminSetting(chatID)
	adminSrc.AnonAdmin = val

	err := UpdateRecordWithZeroValues(&AdminSettings{}, AdminSettings{ChatId: chatID}, AdminSettings{AnonAdmin: val})
	if err != nil {
		log.Errorf("[Database] SetAnonAdminMode: %v - %d", err, chatID)
		return err
	}
	return nil
}
