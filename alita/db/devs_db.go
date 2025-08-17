package db

import (
	"errors"
	"fmt"
	"runtime"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/dustin/go-humanize"
)

// GetTeamMemInfo retrieves developer settings for a user.
// Returns default settings (not a dev) if not found or on error.
func GetTeamMemInfo(userID int64) (devrc *DevSettings) {
	devrc = &DevSettings{}
	err := GetRecord(devrc, DevSettings{UserId: userID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		devrc = &DevSettings{UserId: userID, IsDev: false}
	} else if err != nil {
		devrc = &DevSettings{UserId: userID, IsDev: false}
		log.Errorf("[Database] GetTeamMemInfo: %v - %d", err, userID)
	}
	log.Infof("[Database] GetTeamMemInfo: %d", userID)
	return
}

// GetTeamMembers returns a map of all team members with their roles.
// Currently only supports 'dev' role and returns nil on error.
func GetTeamMembers() map[int64]string {
	var teamArray []*DevSettings
	array := make(map[int64]string)

	err := GetRecords(&teamArray, DevSettings{IsDev: true})
	if err != nil {
		log.Error(err)
		return nil
	}

	for _, result := range teamArray {
		if result.IsDev {
			array[result.UserId] = "dev"
		}
	}

	return array
}

// AddDev adds a user as a developer or updates existing record to dev status.
// Creates a new record if the user doesn't exist in DevSettings.
func AddDev(userID int64) {
	devSettings := &DevSettings{UserId: userID, IsDev: true}

	// Try to update existing record first
	err := UpdateRecord(&DevSettings{}, DevSettings{UserId: userID}, DevSettings{IsDev: true})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new record if not exists
		err = CreateRecord(devSettings)
	}

	if err != nil {
		log.Errorf("[Database] AddDev: %v - %d", err, userID)
		return
	}
	log.Infof("[Database] AddDev: %d", userID)
}

// RemDev removes a user from the developers list by deleting their DevSettings record.
func RemDev(userID int64) {
	err := DB.Where("user_id = ?", userID).Delete(&DevSettings{}).Error
	if err != nil {
		log.Errorf("[Database] RemDev: %v - %d", err, userID)
	}
}

// LoadAllStats generates a comprehensive statistics report for the bot.
// Includes user counts, chat statistics, feature usage, activity metrics, and system information.
func LoadAllStats() string {
	totalUsers := LoadUsersStats()
	activeChats, inactiveChats := LoadChatStats()
	dag, wag, mag := LoadActivityStats()
	dau, wau, mau := LoadUserActivityStats()
	AcCount, ClCount := LoadPinStats()
	uRCount, gRCount := LoadReportStats()
	antiCount := LoadAntifloodStats()
	setRules, pvtRules := LoadRulesStats()
	blacklistTriggers, blacklistChats := LoadBlacklistsStats()
	connectedUsers, connectedChats := LoadConnectionStats()
	disabledCmds, disableEnabledChats := LoadDisableStats()
	filtersNum, filtersChats := LoadFilterStats()
	enabledWelcome, enabledGoodbye, cleanServiceEnabled, cleanWelcomeEnabled, cleanGoodbyeEnabled := LoadGreetingsStats()
	notesNum, notesChats := LoadNotesStats()
	numChannels := LoadChannelStats()

	// Get webhook status information
	var deploymentMode, webhookInfo string
	if config.UseWebhooks {
		deploymentMode = "🌐 Webhook"
		if config.WebhookDomain != "" {
			webhookInfo = fmt.Sprintf("\n    <b>Webhook URL:</b> %s/webhook/***", config.WebhookDomain)
		} else {
			webhookInfo = "\n    <b>Webhook URL:</b> Not configured"
		}
	} else {
		deploymentMode = "🔄 Polling"
		webhookInfo = "\n    <b>Update Method:</b> Long polling"
	}

	result := "<u>Alita's Stats:</u>" +
		fmt.Sprintf("\n\n<b>Deployment Mode:</b> %s%s", deploymentMode, webhookInfo) +
		fmt.Sprintf("\n<b>Go Version:</b> %s", runtime.Version()) +
		fmt.Sprintf("\n<b>Goroutines:</b> %s", humanize.Comma(int64(runtime.NumGoroutine()))) +
		fmt.Sprintf("\n<b>Antiflood:</b> enabled in %s chats", humanize.Comma(antiCount)) +
		fmt.Sprintf(
			"\n<b>Users:</b> %s users found in %s active Chats (%s Inactive, %s Total)",
			humanize.Comma(totalUsers),
			humanize.Comma(int64(activeChats)),
			humanize.Comma(int64(inactiveChats)),
			humanize.Comma(int64(activeChats+inactiveChats)),
		) +
		"\n<b>Group Activity Metrics:</b>" +
		fmt.Sprintf("\n    <b>Daily Active Groups (DAG):</b> %s", humanize.Comma(dag)) +
		fmt.Sprintf("\n    <b>Weekly Active Groups (WAG):</b> %s", humanize.Comma(wag)) +
		fmt.Sprintf("\n    <b>Monthly Active Groups (MAG):</b> %s", humanize.Comma(mag)) +
		"\n<b>User Activity Metrics:</b>" +
		fmt.Sprintf("\n    <b>Daily Active Users (DAU):</b> %s", humanize.Comma(dau)) +
		fmt.Sprintf("\n    <b>Weekly Active Users (WAU):</b> %s", humanize.Comma(wau)) +
		fmt.Sprintf("\n    <b>Monthly Active Users (MAU):</b> %s", humanize.Comma(mau)) +
		"\n<b>Pins:</b>" +
		fmt.Sprintf("\n    <b>CleanLinked Enabled:</b> %s", humanize.Comma(ClCount)) +
		fmt.Sprintf("\n    <b>AntiChannelPin Enabled:</b> %s", humanize.Comma(AcCount)) +
		fmt.Sprintf(
			"\n<b>Reports:</b> %s users enabled reports in %s Chats",
			humanize.Comma(uRCount),
			humanize.Comma(gRCount),
		) +
		"\n<b>Rules:</b>" +
		fmt.Sprintf("\n    <b>Set:</b> %s", humanize.Comma(setRules)) +
		fmt.Sprintf("\n    <b>Private:</b> %s", humanize.Comma(pvtRules)) +
		fmt.Sprintf(
			"\n<b>Blacklists:</b> %s triggers in %s chats",
			humanize.Comma(blacklistTriggers),
			humanize.Comma(blacklistChats),
		) +
		"\n<b>Connections:</b>" +
		fmt.Sprintf("\n    %s users connected to chats", humanize.Comma(connectedUsers)) +
		fmt.Sprintf("\n    %s chats allow user connections", humanize.Comma(connectedChats)) +
		fmt.Sprintf(
			"\n<b>Disabling:</b> %s commands disabled in %s chats",
			humanize.Comma(disabledCmds),
			humanize.Comma(disableEnabledChats),
		) +
		fmt.Sprintf(
			"\n<b>Filters:</b> %s filters saved in %s chats",
			humanize.Comma(filtersNum),
			humanize.Comma(filtersChats),
		) +
		"\n<b>Greetings:</b>" +
		fmt.Sprintf("\n    <b>Welcome Enabled:</b> %s", humanize.Comma(enabledWelcome)) +
		fmt.Sprintf("\n    <b>Goodbye Enabled:</b> %s", humanize.Comma(enabledGoodbye)) +
		fmt.Sprintf("\n    <b>CleanService:</b> %s", humanize.Comma(cleanServiceEnabled)) +
		fmt.Sprintf("\n    <b>CleanWelcome:</b> %s", humanize.Comma(cleanWelcomeEnabled)) +
		fmt.Sprintf("\n    <b>CleanGoodbye:</b> %s", humanize.Comma(cleanGoodbyeEnabled)) +
		fmt.Sprintf(
			"\n<b>Notes:</b> %s notes saved in %s chats",
			humanize.Comma(notesNum),
			humanize.Comma(notesChats),
		) +
		fmt.Sprintf("\n<b>Channels Stored</b>: %s", humanize.Comma(numChannels))

	return result
}
