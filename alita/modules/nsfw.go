package modules

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/jev"
	"github.com/divkix/Alita_Robot/alita/utils/ratelimit"
)

var nsfwModule = moduleStruct{moduleName: "NSFW"}

var newJevClient = func(apiKey string) jev.Client {
	return jev.NewClient(apiKey)
}

func checkNSFW(c *helpers.CommandContext) error {
	if c.Chat == nil || !slices.Contains(config.AppConfig.OwnerChatIDs, c.Chat.Id) ||
		(c.Chat.Type != "group" && c.Chat.Type != "supergroup") {
		return ext.EndGroups
	}
	if config.AppConfig.TypeSafeAPIKey == "" {
		text, _ := c.Tr.GetString("nsfw_no_api_key")
		_, _ = c.Msg.Reply(c.Bot, text, formatting.Shtml())
		return ext.EndGroups
	}

	var textToAnalyze string
	if c.Msg.ReplyToMessage != nil {
		textToAnalyze = c.Msg.ReplyToMessage.Text
		if textToAnalyze == "" {
			textToAnalyze = c.Msg.ReplyToMessage.Caption
		}
	}

	if textToAnalyze == "" {
		args := c.Ctx.Args()
		if len(args) > 1 {
			textToAnalyze = strings.TrimSpace(strings.Join(args[1:], " "))
		}
	}

	if textToAnalyze == "" {
		if c.Msg.ReplyToMessage != nil {
			text, _ := c.Tr.GetString("nsfw_no_text")
			_, _ = c.Msg.Reply(c.Bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		text, _ := c.Tr.GetString("nsfw_no_target")
		_, _ = c.Msg.Reply(c.Bot, text, formatting.Shtml())
		return ext.EndGroups
	}

	if len(textToAnalyze) > 4000 {
		textToAnalyze = textToAnalyze[:4000]
	}
	isAdmin := chat_status.IsUserAdmin(c.Bot, c.Chat.Id, c.User.Id)
	if allowed, wait := ratelimit.GetNSFWRateLimiter().Acquire(c.User.Id, isAdmin, time.Now()); !allowed {
		msg, _ := c.Tr.GetString("nsfw_rate_limited")
		_, _ = c.Msg.Reply(c.Bot, fmt.Sprintf(msg, ratelimit.FormatCooldown((wait+time.Second-1).Truncate(time.Second))), formatting.Shtml())
		return ext.EndGroups
	}

	if c.Chat != nil {
		_, _ = c.Chat.SendAction(c.Bot, "typing", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := newJevClient(config.AppConfig.TypeSafeAPIKey)
	tsResp, err := client.Decide(ctx, textToAnalyze, map[string]jev.Question{
		"is_nsfw":  jev.NewNoulQuestion("Does this text contain NSFW (not safe for work), sexually explicit, pornographic, or adult content?"),
		"is_curse": jev.NewNoulQuestion("Does this text contain profanity, curse words, swear words, or vulgar language?"),
	})
	if err != nil {
		log.WithError(err).Error("[NSFW] TypeSafe AI analysis failed")
		text, _ := c.Tr.GetString("nsfw_api_error")
		_, _ = c.Msg.Reply(c.Bot, text, formatting.Shtml())
		return ext.EndGroups
	}

	nsfwAnswer, nsfwOK := tsResp.Answers["is_nsfw"]
	curseAnswer, curseOK := tsResp.Answers["is_curse"]
	if !nsfwOK || !curseOK || nsfwAnswer.Type != jev.TypeNoul || curseAnswer.Type != jev.TypeNoul ||
		!validNSFWScore(nsfwAnswer.Noul) || !validNSFWScore(curseAnswer.Noul) {
		log.Error("[NSFW] TypeSafe AI returned invalid scores")
		text, _ := c.Tr.GetString("nsfw_api_error")
		_, _ = c.Msg.Reply(c.Bot, text, formatting.Shtml())
		return ext.EndGroups
	}
	nsfwProb := nsfwAnswer.Noul
	curseProb := curseAnswer.Noul

	nsfwPercent := nsfwProb * 100.0
	cursePercent := curseProb * 100.0

	var nsfwStatus string
	if nsfwProb >= 0.5 {
		nsfwStatus, _ = c.Tr.GetString("nsfw_detected")
	} else {
		nsfwStatus, _ = c.Tr.GetString("nsfw_clean")
	}

	var curseStatus string
	if curseProb >= 0.5 {
		curseStatus, _ = c.Tr.GetString("nsfw_detected")
	} else {
		curseStatus, _ = c.Tr.GetString("nsfw_clean")
	}

	var verdict string
	if nsfwProb >= 0.7 || curseProb >= 0.7 {
		verdict, _ = c.Tr.GetString("nsfw_verdict_unsafe")
	} else if nsfwProb >= 0.4 || curseProb >= 0.4 {
		verdict, _ = c.Tr.GetString("nsfw_verdict_flagged")
	} else {
		verdict, _ = c.Tr.GetString("nsfw_verdict_safe")
	}

	resultTemplate, _ := c.Tr.GetString("nsfw_result")
	resultMsg := fmt.Sprintf(resultTemplate, nsfwStatus, nsfwPercent, curseStatus, cursePercent, verdict)

	opts := formatting.Shtml()
	if c.Msg.ReplyToMessage != nil {
		opts.ReplyParameters = &gotgbot.ReplyParameters{
			MessageId:                c.Msg.ReplyToMessage.MessageId,
			AllowSendingWithoutReply: true,
		}
	}

	_, err = c.Msg.Reply(c.Bot, resultMsg, opts)
	if err != nil {
		log.WithError(err).Error("[NSFW] Failed to send analysis result")
		return err
	}

	return ext.EndGroups
}

func validNSFWScore(score float64) bool {
	return !math.IsNaN(score) && score >= 0 && score <= 1
}

// LoadNSFW registers the NSFW and curse words checking command with the dispatcher.
func LoadNSFW(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[nsfwModule.moduleName] = true

	helpers.WrapCommand(
		dispatcher,
		helpers.CommandDescriptor{
			Name:    "nsfw",
			Aliases: []string{"checknsfw", "curse"},
			Group:   0,
		},
		checkNSFW,
	)
}

func init() {
	RegisterLegacyModule("NSFW", 65, LoadNSFW)
}
