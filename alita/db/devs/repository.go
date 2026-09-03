package devs

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"runtime"
	"strconv"
	"strings"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/channels"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/pins"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

// comma formats an int64 with thousands separators (e.g. 1234567 -> "1,234,567").
func comma(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := ""
	if strings.HasPrefix(s, "-") {
		neg, s = "-", s[1:]
	}
	if len(s) <= 3 {
		return neg + s
	}
	// ponytail: simple grouping, fast enough for /stats rendering
	var b strings.Builder
	b.WriteString(neg)
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
		if len(s) > rem {
			b.WriteByte(',')
		}
	}
	for i := rem; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	return b.String()
}

// GetTeamMemInfo retrieves developer settings for a user.
// Returns default settings (not a dev) if not found or on error.
func GetTeamMemInfo(userID int64) (devrc *models.DevSettings) {
	devrc = &models.DevSettings{}
	err := db.GetRecord(devrc, models.DevSettings{UserId: userID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		devrc = &models.DevSettings{UserId: userID, IsDev: false, Sudo: false}
	} else if err != nil {
		devrc = &models.DevSettings{UserId: userID, IsDev: false, Sudo: false}
		log.Errorf("[Database] GetTeamMemInfo: %v - %d", err, userID)
	}
	log.Infof("[Database] GetTeamMemInfo: %d", userID)
	return
}

// GetTeamMembers returns a map of all team members with their roles.
// Queries for both dev and sudo users, combining results.
// A user can have both roles, in which case "dev" takes precedence.
func GetTeamMembers() map[int64]string {
	var devArray []*models.DevSettings
	var sudoArray []*models.DevSettings
	array := make(map[int64]string)

	// Get all dev users
	err := db.GetRecords(&devArray, models.DevSettings{IsDev: true})
	if err != nil {
		log.Error(err)
		return nil
	}

	// Get all sudo users
	err = db.GetRecords(&sudoArray, models.DevSettings{Sudo: true})
	if err != nil {
		log.Error(err)
		return nil
	}

	// First, add sudo users
	for _, result := range sudoArray {
		if result.Sudo {
			array[result.UserId] = "sudo"
		}
	}

	// Then add/override with dev users (dev takes precedence)
	for _, result := range devArray {
		if result.IsDev {
			array[result.UserId] = "dev"
		}
	}

	return array
}

// AddDev adds a user as a developer or updates existing record to dev status.
// Creates a new record if the user doesn't exist in DevSettings.
func AddDev(userID int64) error {
	devSettings := &models.DevSettings{UserId: userID, IsDev: true}

	// Try to update existing record first
	err := db.UpdateRecord(&models.DevSettings{}, models.DevSettings{UserId: userID}, models.DevSettings{IsDev: true})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new record if not exists
		err = db.CreateRecord(devSettings)
	}

	if err != nil {
		log.Errorf("[Database] AddDev: %v - %d", err, userID)
		return err
	}
	log.Infof("[Database] AddDev: %d", userID)
	return nil
}

// RemDev removes developer status from a user by setting IsDev to false.
// Does not delete the record as the user might still have Sudo privileges.
func RemDev(userID int64) error {
	err := db.UpdateRecordWithZeroValues(&models.DevSettings{}, models.DevSettings{UserId: userID}, map[string]any{"is_dev": false})
	if err != nil {
		log.Errorf("[Database] RemDev: %v - %d", err, userID)
		return err
	}
	log.Infof("[Database] RemDev: %d", userID)
	return nil
}

// AddSudo adds a user as a sudo user or updates existing record to sudo status.
// Creates a new record if the user doesn't exist in DevSettings.
func AddSudo(userID int64) error {
	sudoSettings := &models.DevSettings{UserId: userID, Sudo: true}

	// Try to update existing record first
	err := db.UpdateRecord(&models.DevSettings{}, models.DevSettings{UserId: userID}, models.DevSettings{Sudo: true})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new record if not exists
		err = db.CreateRecord(sudoSettings)
	}

	if err != nil {
		log.Errorf("[Database] AddSudo: %v - %d", err, userID)
		return err
	}
	log.Infof("[Database] AddSudo: %d", userID)
	return nil
}

// RemSudo removes sudo status from a user by setting Sudo to false.
// Does not delete the record as the user might still be a Dev.
func RemSudo(userID int64) error {
	err := db.UpdateRecordWithZeroValues(&models.DevSettings{}, models.DevSettings{UserId: userID}, map[string]any{"sudo": false})
	if err != nil {
		log.Errorf("[Database] RemSudo: %v - %d", err, userID)
		return err
	}
	log.Infof("[Database] RemSudo: %d", userID)
	return nil
}

// LoadAllStats generates a comprehensive statistics report for the bot.
// Includes user counts, chat statistics, feature usage (including federations),
// activity metrics, and system information.
func LoadAllStats() string {
	totalUsers := user.LoadUsersStats()
	activeChats, inactiveChats := chats.LoadChatStats()
	dag, wag, mag := chats.LoadActivityStats()
	dau, wau, mau := user.LoadUserActivityStats()
	AcCount, ClCount := pins.LoadPinStats()
	uRCount, gRCount := reports.LoadReportStats()
	antiCount := antiflood.LoadAntifloodStats()
	setRules, pvtRules := rules.LoadRulesStats()
	blacklistTriggers, blacklistChats := blacklists.LoadBlacklistsStats()
	connectedUsers, connectedChats := connections.LoadConnectionStats()
	disabledCmds, disableEnabledChats := disabling.LoadDisableStats()
	filtersNum, filtersChats := filters.LoadFilterStats()
	enabledWelcome, enabledGoodbye, cleanServiceEnabled, cleanWelcomeEnabled, cleanGoodbyeEnabled := greetings.LoadGreetingsStats()
	notesNum, notesChats := notes.LoadNotesStats()
	fedCount, fedChats, fedAdmins, fedBans, fedSubs := federations.LoadFederationStats()
	numChannels := channels.LoadChannelStats()

	// Get webhook status information
	var deploymentMode, webhookInfo string
	if config.AppConfig.UseWebhooks {
		deploymentMode = "🌐 Webhook"
		if config.AppConfig.WebhookDomain != "" {
			webhookInfo = fmt.Sprintf("\n    <b>Webhook URL:</b> %s/webhook/***", config.AppConfig.WebhookDomain)
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
		fmt.Sprintf("\n<b>Goroutines:</b> %s", comma(int64(runtime.NumGoroutine()))) +
		fmt.Sprintf("\n<b>Antiflood:</b> enabled in %s chats", comma(antiCount)) +
		fmt.Sprintf(
			"\n<b>Users:</b> %s users found in %s active Chats (%s Inactive, %s Total)",
			comma(totalUsers),
			comma(int64(activeChats)),
			comma(int64(inactiveChats)),
			comma(int64(activeChats+inactiveChats)),
		) +
		"\n<b>Group Activity Metrics:</b>" +
		fmt.Sprintf("\n    <b>Daily Active Groups (DAG):</b> %s", comma(dag)) +
		fmt.Sprintf("\n    <b>Weekly Active Groups (WAG):</b> %s", comma(wag)) +
		fmt.Sprintf("\n    <b>Monthly Active Groups (MAG):</b> %s", comma(mag)) +
		"\n<b>User Activity Metrics:</b>" +
		fmt.Sprintf("\n    <b>Daily Active Users (DAU):</b> %s", comma(dau)) +
		fmt.Sprintf("\n    <b>Weekly Active Users (WAU):</b> %s", comma(wau)) +
		fmt.Sprintf("\n    <b>Monthly Active Users (MAU):</b> %s", comma(mau)) +
		"\n<b>Pins:</b>" +
		fmt.Sprintf("\n    <b>CleanLinked Enabled:</b> %s", comma(ClCount)) +
		fmt.Sprintf("\n    <b>AntiChannelPin Enabled:</b> %s", comma(AcCount)) +
		fmt.Sprintf(
			"\n<b>Reports:</b> %s users enabled reports in %s Chats",
			comma(uRCount),
			comma(gRCount),
		) +
		"\n<b>Rules:</b>" +
		fmt.Sprintf("\n    <b>Set:</b> %s", comma(setRules)) +
		fmt.Sprintf("\n    <b>Private:</b> %s", comma(pvtRules)) +
		fmt.Sprintf(
			"\n<b>Blacklists:</b> %s triggers in %s chats",
			comma(blacklistTriggers),
			comma(blacklistChats),
		) +
		"\n<b>Connections:</b>" +
		fmt.Sprintf("\n    %s users connected to chats", comma(connectedUsers)) +
		fmt.Sprintf("\n    %s chats allow user connections", comma(connectedChats)) +
		fmt.Sprintf(
			"\n<b>Disabling:</b> %s commands disabled in %s chats",
			comma(disabledCmds),
			comma(disableEnabledChats),
		) +
		fmt.Sprintf(
			"\n<b>Filters:</b> %s filters saved in %s chats",
			comma(filtersNum),
			comma(filtersChats),
		) +
		"\n<b>Greetings:</b>" +
		fmt.Sprintf("\n    <b>Welcome Enabled:</b> %s", comma(enabledWelcome)) +
		fmt.Sprintf("\n    <b>Goodbye Enabled:</b> %s", comma(enabledGoodbye)) +
		fmt.Sprintf("\n    <b>CleanService:</b> %s", comma(cleanServiceEnabled)) +
		fmt.Sprintf("\n    <b>CleanWelcome:</b> %s", comma(cleanWelcomeEnabled)) +
		fmt.Sprintf("\n    <b>CleanGoodbye:</b> %s", comma(cleanGoodbyeEnabled)) +
		fmt.Sprintf(
			"\n<b>Notes:</b> %s notes saved in %s chats",
			comma(notesNum),
			comma(notesChats),
		) +
		"\n<b>Federations:</b>" +
		fmt.Sprintf("\n    <b>Total:</b> %s", comma(fedCount)) +
		fmt.Sprintf("\n    <b>Chats:</b> %s", comma(fedChats)) +
		fmt.Sprintf("\n    <b>Admins:</b> %s", comma(fedAdmins)) +
		fmt.Sprintf("\n    <b>Bans:</b> %s", comma(fedBans)) +
		fmt.Sprintf("\n    <b>Subscriptions:</b> %s", comma(fedSubs)) +
		fmt.Sprintf("\n<b>Channels Stored</b>: %s", comma(numChannels))

	return result
}
