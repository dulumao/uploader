package common

import (
	"os"
	"time"
)

type Object struct {
	Path         string
	Name         string
	LastModified *time.Time
}

func GetSize(f *os.File) (int64, error) {
	fileInfo, err := f.Stat()

	if err != nil {
		return 0, err
	}

	return fileInfo.Size(), nil
}
