package utils

import (
	"fmt"
	"os"
)

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
