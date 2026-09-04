package admin

import (
	"errors"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func GetAdminSettings(chatID int64) *models.AdminSettings {
	return checkAdminSetting(chatID)
}

func checkAdminSetting(chatID int64) (adminSrc *models.AdminSettings) {
	adminSrc = &models.AdminSettings{}

	err := db.GetRecord(adminSrc, models.AdminSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		adminSrc = &models.AdminSettings{ChatId: chatID, AnonAdmin: false}
		err := db.CreateRecord(adminSrc)
		if err != nil {
			log.Errorf("[Database][checkAdminSetting]: %v ", err)
		}
	} else if err != nil {
		adminSrc = &models.AdminSettings{ChatId: chatID, AnonAdmin: false}
		log.Errorf("[Database][checkAdminSetting]: %v ", err)
	}
	return adminSrc
}

func SetAnonAdminMode(chatID int64, val bool) error {
	adminSrc := checkAdminSetting(chatID)
	adminSrc.AnonAdmin = val

	err := db.UpdateRecordWithZeroValues(&models.AdminSettings{}, models.AdminSettings{ChatId: chatID}, map[string]any{"anon_admin": val})
	if err != nil {
		log.Errorf("[Database] SetAnonAdminMode: %v - %d", err, chatID)
		return err
	}
	return nil
}
