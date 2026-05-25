package launcher

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var ErrRootFallback = errors.New("refusing to fallback to root identity")

func SelectIdentity(desiredUID, desiredGID, observedUID, observedGID int, chownErr error) (int, int, error) {
	if chownErr == nil {
		return desiredUID, desiredGID, nil
	}

	if errors.Is(chownErr, fs.ErrPermission) || errors.Is(chownErr, syscall.EPERM) {
		if observedUID == 0 {
			return 0, 0, ErrRootFallback
		}
		return observedUID, observedGID, nil
	}

	return 0, 0, chownErr
}

func EnsureIdentityFiles(passwdPath, groupPath string, uid, gid int) error {
	passwdData, err := os.ReadFile(passwdPath)
	if err != nil {
		return err
	}
	groupData, err := os.ReadFile(groupPath)
	if err != nil {
		return err
	}

	if !hasGroupID(groupData, gid) {
		groupLine := fmt.Sprintf("ociger:x:%d:\n", gid)
		if err := appendLine(groupPath, groupLine); err != nil {
			return err
		}
	}

	if !hasUserID(passwdData, uid) {
		passwdLine := fmt.Sprintf("ociger:x:%d:%d::/tmp:/sbin/nologin\n", uid, gid)
		if err := appendLine(passwdPath, passwdLine); err != nil {
			return err
		}
	}

	return nil
}

func appendLine(path string, line string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(line)
	return err
}

func hasUserID(contents []byte, uid int) bool {
	return hasNumericField(contents, uid, 2)
}

func hasGroupID(contents []byte, gid int) bool {
	return hasNumericField(contents, gid, 2)
}

func hasNumericField(contents []byte, target int, index int) bool {
	for _, line := range bytes.Split(contents, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		parts := strings.Split(string(line), ":")
		if len(parts) <= index {
			continue
		}
		value, err := strconv.Atoi(parts[index])
		if err != nil {
			continue
		}
		if value == target {
			return true
		}
	}
	return false
}
