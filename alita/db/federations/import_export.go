package federations

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
)

// supported export/import formats.
const (
	FormatCSV     = "csv"
	FormatMiniCSV = "minicsv"
	FormatJSON    = "json"
)

// ExportBans exports a federation's ban list in the requested format.
// The returned string is the suggested filename extension.
func ExportBans(b *gotgbot.Bot, fedID string, format string) ([]byte, string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = FormatCSV
	}

	switch format {
	case FormatCSV:
		return exportCSV(b, fedID, false)
	case FormatMiniCSV:
		return exportCSV(b, fedID, true)
	case FormatJSON:
		return exportJSON(b, fedID)
	default:
		return nil, "", fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportRow represents a single exported ban row.
type exportRow struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"user_name"`
	Reason    string `json:"reason"`
}

func exportCSV(b *gotgbot.Bot, fedID string, mini bool) ([]byte, string, error) {
	bans, err := ListBans(fedID)
	if err != nil {
		return nil, "", err
	}

	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)

	if mini {
		_ = writer.Write([]string{"id", "reason"})
	} else {
		_ = writer.Write([]string{"id", "firstname", "lastname", "username", "reason"})
	}

	for _, ban := range bans {
		row := resolveExportRow(b, ban.UserID, ban.Reason)
		if mini {
			_ = writer.Write([]string{
				strconv.FormatInt(row.UserID, 10),
				row.Reason,
			})
		} else {
			_ = writer.Write([]string{
				strconv.FormatInt(row.UserID, 10),
				row.FirstName,
				row.LastName,
				row.Username,
				row.Reason,
			})
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), "csv", nil
}

func exportJSON(b *gotgbot.Bot, fedID string) ([]byte, string, error) {
	bans, err := ListBans(fedID)
	if err != nil {
		return nil, "", err
	}

	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)

	for _, ban := range bans {
		row := resolveExportRow(b, ban.UserID, ban.Reason)
		if err := encoder.Encode(row); err != nil {
			return nil, "", err
		}
	}

	return buf.Bytes(), "json", nil
}

func resolveExportRow(b *gotgbot.Bot, userID int64, reason string) exportRow {
	row := exportRow{UserID: userID, Reason: reason}
	if b == nil {
		return row
	}

	chat, err := b.GetChat(userID, nil)
	if err != nil {
		return row
	}
	info := chat.ToChat()
	row.FirstName = info.FirstName
	row.LastName = info.LastName
	if info.Username != "" {
		row.Username = info.Username
	}
	return row
}

// ImportBans imports bans into a federation from CSV or NDJSON/JSONL data.
func ImportBans(fedID string, data []byte, format string) (imported int, skipped int, err error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case FormatCSV:
		return importCSV(fedID, data)
	case FormatJSON:
		return importJSON(fedID, data)
	default:
		return 0, 0, fmt.Errorf("unsupported import format: %s", format)
	}
}

func importCSV(fedID string, data []byte) (int, int, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) == 0 {
		return 0, 0, nil
	}

	// Detect id column index from header.
	idIdx := -1
	reasonIdx := -1
	for i, h := range records[0] {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "id", "user_id":
			idIdx = i
		case "reason":
			reasonIdx = i
		}
	}
	if idIdx == -1 {
		return 0, 0, errors.New("CSV header missing id/user_id column")
	}

	imported := 0
	skipped := 0
	for _, record := range records[1:] {
		if len(record) <= idIdx {
			skipped++
			continue
		}

		userID, err := strconv.ParseInt(strings.TrimSpace(record[idIdx]), 10, 64)
		if err != nil || userID <= 0 {
			skipped++
			continue
		}

		reason := ""
		if reasonIdx != -1 && len(record) > reasonIdx {
			reason = strings.TrimSpace(record[reasonIdx])
		}

		if err := BanUser(fedID, userID, reason, 0); err != nil {
			log.Debugf("[Federations] ImportBans skipping user %d: %v", userID, err)
			skipped++
			continue
		}
		imported++
	}

	return imported, skipped, nil
}

func importJSON(fedID string, data []byte) (int, int, error) {
	imported := 0
	skipped := 0

	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var row exportRow
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			// Try decoding as a map for flexibility.
			var raw map[string]any
			if decodeErr := json.Unmarshal([]byte(err.Error()), &raw); decodeErr != nil {
				skipped++
				continue
			}
			row = mapToExportRow(raw)
		}

		if row.UserID <= 0 {
			skipped++
			continue
		}

		if err := BanUser(fedID, row.UserID, row.Reason, 0); err != nil {
			log.Debugf("[Federations] ImportBans skipping user %d: %v", row.UserID, err)
			skipped++
			continue
		}
		imported++
	}

	return imported, skipped, nil
}

func mapToExportRow(raw map[string]any) exportRow {
	var row exportRow
	if v, ok := raw["user_id"].(float64); ok {
		row.UserID = int64(v)
	} else if v, ok := raw["id"].(float64); ok {
		row.UserID = int64(v)
	}
	if v, ok := raw["first_name"].(string); ok {
		row.FirstName = v
	}
	if v, ok := raw["last_name"].(string); ok {
		row.LastName = v
	}
	if v, ok := raw["user_name"].(string); ok {
		row.Username = v
	} else if v, ok := raw["username"].(string); ok {
		row.Username = v
	}
	if v, ok := raw["reason"].(string); ok {
		row.Reason = v
	}
	return row
}
