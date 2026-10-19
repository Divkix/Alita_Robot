package db

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

func EnsureUserInDb(userId int64, username, firstName string) error {
	userUpdate := &User{
		UserId:   userId,
		UserName: username,
		Name:     firstName,
	}
	result := DB.Where("user_id = ?", userId).Assign(userUpdate).FirstOrCreate(&User{})
	if result.Error != nil {
		log.Errorf("[Database] EnsureUserInDb: %v", result.Error)
		return fmt.Errorf("failed to ensure user %d in database: %w", userId, result.Error)
	}
	return nil
}
