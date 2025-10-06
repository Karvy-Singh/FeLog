package utils

import (
	"fmt"
	"log"
	"os"
)

func MakeLogger(name string) (*log.Logger, error) {
	_ = os.Remove(fmt.Sprintf("./%s.txt", name))

	f, err := os.OpenFile(fmt.Sprintf("./%s.txt", name),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return nil, fmt.Errorf("log file: %w", err)
	}

	logger := log.New(f, "", log.LstdFlags|log.Lmicroseconds)

	return logger, nil
}
