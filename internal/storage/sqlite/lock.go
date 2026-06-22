package sqlite

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gitlab.digital-spirit.ru/solutions/common/kan/internal/config"
	"gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"
)

type Lock struct {
	file *os.File
}

func AcquireLock(databasePath string) (*Lock, error) {
	return acquireLock(databasePath, 0)
}

func AcquireLockTimeout(databasePath string, timeout time.Duration) (*Lock, error) {
	return acquireLock(databasePath, timeout)
}

func acquireLock(databasePath string, timeout time.Duration) (*Lock, error) {
	path := databasePath + ".lock"
	if err := config.EnsureParent(path); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	deadline := time.Now().Add(timeout)
	for {
		var acquired bool
		acquired, err = tryLock(file)
		if err == nil {
			if acquired {
				return &Lock{file: file}, nil
			}
		} else {
			file.Close()
			return nil, fmt.Errorf("lock database: %w", err)
		}
		if timeout <= 0 || time.Now().After(deadline) {
			file.Close()
			return nil, domain.ErrLocked
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (lock *Lock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	file := lock.file
	lock.file = nil
	unlockErr := unlockFile(file)
	closeErr := file.Close()
	return errors.Join(unlockErr, closeErr)
}
