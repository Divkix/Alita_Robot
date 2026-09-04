package config

import (
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

type typeConvertor struct {
	str string
}

func (t typeConvertor) StringArray() []string {
	allUpdates := strings.Split(t.str, ",")
	for i, j := range allUpdates {
		allUpdates[i] = strings.TrimSpace(j)
	}
	return allUpdates
}

func (t typeConvertor) Int() int {
	if t.str == "" {
		return 0
	}
	val, err := strconv.Atoi(t.str)
	if err != nil {
		log.WithError(err).WithField("value", t.str).Warn("Failed to convert config value to int")
		return 0
	}
	return val
}

func (t typeConvertor) Int64() int64 {
	if t.str == "" {
		return 0
	}
	val, err := strconv.ParseInt(t.str, 10, 64)
	if err != nil {
		log.WithError(err).WithField("value", t.str).Warn("Failed to convert config value to int64")
		return 0
	}
	return val
}

func (t typeConvertor) Bool() bool {
	lower := strings.ToLower(strings.TrimSpace(t.str))
	return lower == "yes" || lower == "true" || lower == "1"
}
