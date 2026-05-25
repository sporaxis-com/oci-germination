package launcher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestSelectIdentityFallsBackToObservedOwnerOnPermissionDenied(t *testing.T) {
	uid, gid, err := SelectIdentity(999, 999, 501, 20, syscall.EPERM)
	if err != nil {
		t.Fatalf("SelectIdentity returned error: %v", err)
	}

	if uid != 501 || gid != 20 {
		t.Fatalf("expected fallback identity 501:20, got %d:%d", uid, gid)
	}
}

func TestSelectIdentityKeepsDesiredOwnerWhenChownSucceeds(t *testing.T) {
	uid, gid, err := SelectIdentity(999, 999, 501, 20, nil)
	if err != nil {
		t.Fatalf("SelectIdentity returned error: %v", err)
	}

	if uid != 999 || gid != 999 {
		t.Fatalf("expected desired identity 999:999, got %d:%d", uid, gid)
	}
}

func TestSelectIdentityRejectsRootFallback(t *testing.T) {
	_, _, err := SelectIdentity(999, 999, 0, 0, syscall.EPERM)
	if !errors.Is(err, ErrRootFallback) {
		t.Fatalf("expected ErrRootFallback, got %v", err)
	}
}

func TestEnsureIdentityFilesAppendsMissingEntries(t *testing.T) {
	tempDir := t.TempDir()
	passwdPath := filepath.Join(tempDir, "passwd")
	groupPath := filepath.Join(tempDir, "group")

	if err := os.WriteFile(passwdPath, []byte("root:x:0:0:root:/root:/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(groupPath, []byte("root:x:0:\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureIdentityFiles(passwdPath, groupPath, 501, 20); err != nil {
		t.Fatalf("EnsureIdentityFiles returned error: %v", err)
	}

	passwdData, err := os.ReadFile(passwdPath)
	if err != nil {
		t.Fatal(err)
	}
	groupData, err := os.ReadFile(groupPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(passwdData), "ociger:x:501:20::/tmp:/sbin/nologin") {
		t.Fatalf("passwd entry missing:\n%s", passwdData)
	}
	if !strings.Contains(string(groupData), "ociger:x:20:") {
		t.Fatalf("group entry missing:\n%s", groupData)
	}
}
