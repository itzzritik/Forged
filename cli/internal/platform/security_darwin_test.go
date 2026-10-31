package platform

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"
)

func TestSecurityDeleteAllGenericPasswords(t *testing.T) {
	ctx := context.Background()
	service := fmt.Sprintf("com.forged.test-purge-%d", time.Now().UnixNano())
	for _, account := range []string{"old-install-a", "old-install-b", "current"} {
		if out, err := SecurityAddGenericPassword(ctx, service, account, "x"); err != nil {
			t.Skipf("keychain unavailable: %v %s", err, out)
		}
	}
	if err := exec.Command("security", "find-generic-password", "-s", service).Run(); err != nil {
		t.Skipf("keychain did not store the test items: %v", err)
	}

	if err := SecurityDeleteAllGenericPasswords(ctx, service); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("security", "find-generic-password", "-s", service).Run(); err == nil {
		t.Fatalf("items for %s are still in the keychain", service)
	}
}
