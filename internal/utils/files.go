package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"
)

const ConfigFileName string = "config.toml"
const ConfigDirName string = "nutmix"
const logFileName string = "nutmix.log"

var (
	DOCKER_ENV           = "DOCKER"
	MODE_ENV             = "MODE"
	MINT_PRIVATE_KEY_ENV = "MINT_PRIVATE_KEY"
)

type SlogRecordJSON struct {
	Time  time.Time
	Msg   string
	Level slog.Level
}

func ParseLogFileByLevelAndTime(file *os.File, wantedLevel []slog.Level, limitTime time.Time) []SlogRecordJSON {

	var logRecords []SlogRecordJSON
	scanner := bufio.NewScanner(file)

	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {

		lineBytes := scanner.Bytes()
		var logRecord SlogRecordJSON
		err := json.Unmarshal(lineBytes, &logRecord)
		if err != nil {
			continue
		}

		if slices.Contains(wantedLevel, logRecord.Level) && logRecord.Time.Unix() > limitTime.Unix() {
			logRecords = append(logRecords, logRecord)
		}

	}

	return logRecords
}

func GetLogsDirectory() (string, error) {
	dir, err := os.UserConfigDir()

	if err != nil {
		return "", fmt.Errorf("Could not get config dir: %w", err)
	}
	var pathToProjectDir string = dir + "/" + ConfigDirName

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
