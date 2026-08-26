package modules

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

var errTelegramFileTooLarge = fmt.Errorf("file too large")

func parseFedBanFile(name string, data []byte) ([]models.FederationBan, error) {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".json"), strings.HasSuffix(lower, ".jsonl"), strings.HasSuffix(lower, ".ndjson"):
		return parseFedBanJSON(data)
	default:
		return parseFedBanCSV(data)
	}
}

func parseFedBanCSV(data []byte) ([]models.FederationBan, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("empty csv")
	}
	header := make([]string, len(records[0]))
	for i, col := range records[0] {
		header[i] = strings.ToLower(strings.TrimSpace(col))
	}
	idIdx, reasonIdx := -1, -1
	for i, col := range header {
		switch col {
		case "id", "user_id", "userid":
			idIdx = i
		case "reason":
			reasonIdx = i
		}
	}
	start := 1
	if idIdx < 0 {
		// Treat the first row as data if there is no header.
		idIdx = 0
		start = 0
	}
	out := make([]models.FederationBan, 0, len(records)-start)
	for _, rec := range records[start:] {
		if idIdx >= len(rec) {
			continue
		}
		userID, err := strconv.ParseInt(strings.TrimSpace(rec[idIdx]), 10, 64)
		if err != nil || userID <= 0 {
			continue
		}
		reason := ""
		if reasonIdx >= 0 && reasonIdx < len(rec) {
			reason = strings.TrimSpace(rec[reasonIdx])
		}
		out = append(out, models.FederationBan{UserID: userID, Reason: reason})
	}
	return out, nil
}

func parseFedBanJSON(data []byte) ([]models.FederationBan, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty json")
	}
	if trimmed[0] == '[' {
		var rows []map[string]any
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, err
		}
		out := make([]models.FederationBan, 0, len(rows))
		for _, row := range rows {
			if ban, ok := mapToFedBan(row); ok {
				out = append(out, ban)
			}
		}
		return out, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	out := make([]models.FederationBan, 0)
	for {
		var row map[string]any
		if err := decoder.Decode(&row); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if ban, ok := mapToFedBan(row); ok {
			out = append(out, ban)
		}
	}
	return out, nil
}

func mapToFedBan(row map[string]any) (models.FederationBan, bool) {
	raw, ok := row["user_id"]
	if !ok {
		raw, ok = row["id"]
	}
	if !ok {
		return models.FederationBan{}, false
	}
	userID, ok := anyToInt64(raw)
	if !ok || userID <= 0 {
		return models.FederationBan{}, false
	}
	reason, _ := row["reason"].(string)
	return models.FederationBan{UserID: userID, Reason: reason}, true
}

func anyToInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		return i, err == nil
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func formatFedBanCSV(bans []models.FederationBan, mini bool) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if mini {
		_ = w.Write([]string{"id", "reason"})
		for _, ban := range bans {
			_ = w.Write([]string{strconv.FormatInt(ban.UserID, 10), ban.Reason})
		}
	} else {
		_ = w.Write([]string{"id", "reason", "banned_by"})
		for _, ban := range bans {
			_ = w.Write([]string{
				strconv.FormatInt(ban.UserID, 10),
				ban.Reason,
				strconv.FormatInt(ban.BannedBy, 10),
			})
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func formatFedBanJSONL(bans []models.FederationBan) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, ban := range bans {
		if err := enc.Encode(map[string]any{
			"user_id":   ban.UserID,
			"reason":    ban.Reason,
			"banned_by": ban.BannedBy,
		}); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func downloadTelegramFile(b *gotgbot.Bot, fileID string) ([]byte, error) {
	if b == nil || fileID == "" {
		return nil, fmt.Errorf("missing file")
	}
	file, err := b.GetFile(fileID, &gotgbot.GetFileOpts{})
	if err != nil {
		return nil, err
	}
	baseURL, err := url.Parse(backupDownloadBaseURL)
	if err != nil {
		return nil, err
	}
	downloadURL, err := url.Parse(fmt.Sprintf("%s%s/%s", backupDownloadBaseURL, b.Token, file.FilePath))
	if err != nil {
		return nil, err
	}
	if downloadURL.Scheme != baseURL.Scheme || downloadURL.Host != baseURL.Host {
		return nil, fmt.Errorf("unexpected download host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := backupDownloadHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBackupFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBackupFileSize {
		return nil, errTelegramFileTooLarge
	}
	return data, nil
}

func downloadFedBanDocument(b *gotgbot.Bot, doc *gotgbot.Document, tr *i18n.Translator) ([]byte, string) {
	fail, _ := tr.GetString("feds_import_failed")
	if doc.FileSize > maxBackupFileSize {
		return nil, fail
	}
	data, err := downloadTelegramFile(b, doc.FileId)
	if err != nil {
		log.Errorf("[Federations] download: %v", err)
		return nil, fail
	}
	return data, ""
}
