package modules

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/jev"
)

var nsfwModule = moduleStruct{moduleName: "NSFW"}

var newJevClient = func(apiKey string) jev.Client {
	return jev.NewClient(apiKey)
}

func checkNSFW(c *helpers.CommandContext) error {
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
		apiErrTemplate, _ := c.Tr.GetString("nsfw_api_error")
		_, _ = c.Msg.Reply(c.Bot, fmt.Sprintf(apiErrTemplate, html.EscapeString(err.Error())), formatting.Shtml())
		return ext.EndGroups
	}

	nsfwProb := tsResp.Answers["is_nsfw"].Noul
	curseProb := tsResp.Answers["is_curse"].Noul

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

// LoadNSFW registers the NSFW and curse words checking command with the dispatcher.
func LoadNSFW(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[nsfwModule.moduleName] = true

	helpers.WrapCommand(
		dispatcher,
		helpers.CommandDescriptor{
			Name:    "nsfw",
			Aliases: []string{"checknsfw", "curse"},
			Group:   0,
			RequiredChecks: []helpers.CheckFunc{
				helpers.RequireGroup(),
				helpers.RequireUserAdmin(),
			},
		},
		checkNSFW,
	)
}

func init() {
	RegisterLegacyModule("NSFW", 65, LoadNSFW)
}
