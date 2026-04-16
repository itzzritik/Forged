package agent

import (
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type fakeRouteSessions struct {
	allowed map[int][]string
	used    map[int][]string
}

func (f *fakeRouteSessions) AllowedFingerprints(clientPID int) []string {
	return append([]string(nil), f.allowed[clientPID]...)
}

func (f *fakeRouteSessions) RecordSignature(clientPID int, fingerprint string) {
	if f.used == nil {
		f.used = map[int][]string{}
	}
	f.used[clientPID] = append(f.used[clientPID], fingerprint)
}

func newScopedAgent(t *testing.T) (*ForgedAgent, *fakeRouteSessions, []vault.Key) {
	t.Helper()

	v, err := vault.Create(filepath.Join(t.TempDir(), "vault.forged"), []byte("correct horse battery staple"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	t.Cleanup(v.Close)

	v.Data.Metadata.DeviceID = "device-1"
	ks := vault.NewKeyStore(v)

	keyA, err := ks.Generate("a", "test-a")
	if err != nil {
		t.Fatalf("generate key a: %v", err)
	}
	keyB, err := ks.Generate("b", "test-b")
	if err != nil {
		t.Fatalf("generate key b: %v", err)
	}

	routes := &fakeRouteSessions{}
	base := New(ks)
	base.SetRouteSessions(routes)

	return base, routes, []vault.Key{keyA, keyB}
}

func TestSessionAgentListFiltersKeysToAllowedFingerprints(t *testing.T) {
	base, routes, keys := newScopedAgent(t)
	routes.allowed = map[int][]string{4242: []string{keys[1].Fingerprint}}
	session := base.ForClientPID(4242)

	got, err := session.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Comment != keys[1].Name {
		t.Fatalf("unexpected filtered keys: %#v", got)
	}
}

func TestSessionAgentSignRecordsUsedFingerprint(t *testing.T) {
	base, routes, keys := newScopedAgent(t)
	routes.allowed = map[int][]string{4242: []string{keys[1].Fingerprint}}
	session := base.ForClientPID(4242)
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keys[1].PublicKey))
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}

	if _, err := session.Sign(pub, []byte("payload")); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(routes.used[4242]) != 1 || routes.used[4242][0] != keys[1].Fingerprint {
		t.Fatalf("unexpected recorded fingerprints: %#v", routes.used)
	}
}
