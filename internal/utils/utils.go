package utils

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

func NewCustomLogger() *slog.Logger {
	return slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if (a.Key == slog.TimeKey || a.Key == slog.LevelKey) && len(groups) == 0 {
					return slog.Attr{}
				}
				return a
			},
		}),
	)
}

func ConvertIntToPath(i int) (string, error) {
	// Convert to a string using numerical base equal to ABC length
	path := strconv.FormatInt(int64(i), len(ABC))

	if len(path) > 4 {
		return "", errors.New("cannot convert int to str. supplied integer is too large")
	}

	// left-pad with "0" until it reaches length of 4
	path = strings.ToUpper(padStart(path, "0", (STEP_LENGTH - len(path))))
	return path, nil
}

func ConvertPathToInt(path string) (int, error) {
	num, err := strconv.ParseInt(path, len(ABC), 0)
	if err != nil {
		return 0, err
	}
	return int(num), nil
}

func IncrementPath(path string) (string, error) {
	// Exclude the last 4 characters, which is the latest root path, to get parent path
	parentPath := path[0 : len(path)-STEP_LENGTH]

	// Parse only the latest root path (last 4 characters of path)
	stepInt, err := ConvertPathToInt(path[len(path)-4:])
	if err != nil {
		return "", err
	}

	// Get incremented path string
	newPath, err := ConvertIntToPath(stepInt + 1)
	if err != nil {
		return "", err
	}

	// append incremented path to parentPath
	return parentPath + newPath, nil
}

func padStart(s, fill string, count int) string {
	return strings.Repeat(fill, count) + s
}
