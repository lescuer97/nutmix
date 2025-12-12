package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"
)

const LogExtraInfo string = "extra-info"

var (
	DOCKER_ENV           = "DOCKER"
	MODE_ENV             = "MODE"
	MINT_PRIVATE_KEY_ENV = "MINT_PRIVATE_KEY"
)

type SlogRecordJSON struct {
	Time      time.Time
	Msg       string
	Level     slog.Level
	ExtraInfo string         `json:"extra-info"`
	Extras    map[string]any `json:"-"`
}

// ParseLogFileByLevelAndTime parses the provided log file line-by-line assuming the file is in ascending time order
// (oldest logs first). It skips lines until it finds the first log with Time >= limitTime, then collects all
// subsequent records whose Level is in wantedLevel.
func ParseLogFileByLevelAndTime(file *os.File, wantedLevel []slog.Level, limitTime time.Time) []SlogRecordJSON {
	var logRecords []SlogRecordJSON
	scanner := bufio.NewScanner(file)

	started := false // become true once we hit records >= limitTime

	for scanner.Scan() {
		lineBytes := scanner.Bytes()

		// unmarshal generically first
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(lineBytes, &raw); err != nil {
			continue
		}

		var logRecord SlogRecordJSON

		// extract time (try RFC3339 string or unix int)
		if tRaw, ok := raw["time"]; ok {
			var tStr string
			if err := json.Unmarshal(tRaw, &tStr); err == nil {
				// try common timestamp layouts
				if parsed, err := time.Parse(time.RFC3339, tStr); err == nil {
					logRecord.Time = parsed
				} else if parsed, err := time.Parse(time.RFC3339Nano, tStr); err == nil {
					logRecord.Time = parsed
				}
			} else {
				var unix int64
				if err := json.Unmarshal(tRaw, &unix); err == nil {
					logRecord.Time = time.Unix(unix, 0)
				}
			}
			delete(raw, "time")
		}

		// extract msg
		if mRaw, ok := raw["msg"]; ok {
			_ = json.Unmarshal(mRaw, &logRecord.Msg)
			delete(raw, "msg")
		}

		// extract level
		if lRaw, ok := raw["level"]; ok {
			var lStr string
			if err := json.Unmarshal(lRaw, &lStr); err == nil {
				switch strings.ToLower(lStr) {
				case "debug":
					logRecord.Level = slog.LevelDebug
				case "info":
					logRecord.Level = slog.LevelInfo
				case "warn", "warning":
					logRecord.Level = slog.LevelWarn
				case "error":
					logRecord.Level = slog.LevelError
				}
			}
			delete(raw, "level")
		}

		// keep existing extra-info if present
		if eiRaw, ok := raw["extra-info"]; ok {
			var eiStr string
			if err := json.Unmarshal(eiRaw, &eiStr); err == nil {
				logRecord.ExtraInfo = eiStr
			}
			delete(raw, "extra-info")
		}

		// anything left is extras
		if len(raw) > 0 {
			logRecord.Extras = make(map[string]any)
			for k, v := range raw {
				var val any
				_ = json.Unmarshal(v, &val)
				logRecord.Extras[k] = val
			}
			// also store a compact JSON string for templ rendering
			if b, err := json.Marshal(logRecord.Extras); err == nil {
				if logRecord.ExtraInfo != "" {
					logRecord.ExtraInfo = logRecord.ExtraInfo + " " + string(b)
				} else {
					logRecord.ExtraInfo = string(b)
				}
			}
		}

		// assume ascending order: skip until we reach logs newer than or equal to limitTime
		if logRecord.Time.IsZero() {
			// cannot determine time, include only if we've started and level matches
			if started && slices.Contains(wantedLevel, logRecord.Level) {
				logRecords = append(logRecords, logRecord)
			}
			continue
		}

		if !started {
			if logRecord.Time.After(limitTime) || logRecord.Time.Equal(limitTime) {
				started = true
				if slices.Contains(wantedLevel, logRecord.Level) {
					logRecords = append(logRecords, logRecord)
				}
			}
		} else {
			// already in window, include if level matches
			if (logRecord.Time.After(limitTime) || logRecord.Time.Equal(limitTime)) && slices.Contains(wantedLevel, logRecord.Level) {
				logRecords = append(logRecords, logRecord)
			}
		}
	}

	return logRecords
}

func GetLogsDirectory() (string, error) {
	dir, err := os.UserConfigDir()

	if err != nil {
		return "", fmt.Errorf("could not get config dir: %w", err)
	}
	var pathToProjectDir = dir + "/" + ConfigDirName

	if os.Getenv(DOCKER_ENV) == "true" {
		pathToProjectDir = "/var/log/nutmix"
	}

	return pathToProjectDir, nil
}

func CreateDirectoryAndPath(dirPath string, filename string) error {

	completeFilePath := dirPath + "/" + filename

	_, err := os.Stat(dirPath)

	if os.IsNotExist(err) {
		err = os.MkdirAll(dirPath, 0764)
		if err != nil {
			return fmt.Errorf("os.MkdirAll(pathToProjectDir, 0764) %w", err)
		}
	}

	_, err = os.Stat(completeFilePath)

	if os.IsNotExist(err) {
		_, err := os.Create(completeFilePath)
		if err != nil {
			return fmt.Errorf("os.Create(pathToProjectConfigFile) %w", err)
		}
	}

	return nil

}
