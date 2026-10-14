//go:build linux

package daemon

import (
	"slices"
	"testing"
)

func TestSystemctlPropertiesAnyOrder(t *testing.T) {
	props := systemctlProperties([]byte("MainPID=4242\nLoadState=loaded\nSubState=running\nActiveState=active\n"))
	if props["MainPID"] != "4242" || props["LoadState"] != "loaded" || props["ActiveState"] != "active" || props["SubState"] != "running" {
		t.Fatalf("got %v", props)
	}
}

func TestParseSystemdCommand(t *testing.T) {
	cases := map[string][]string{
		`"/home/a b/forged" "daemon"`:  {"/home/a b/forged", "daemon"},
		`'/opt/forged' daemon`:         {"/opt/forged", "daemon"},
		`/usr/bin/forged   daemon`:     {"/usr/bin/forged", "daemon"},
		`"/home/100%%/forged" daemon`:  {"/home/100%/forged", "daemon"},
		`"/home/q\"x/forged" "daemon"`: {`/home/q"x/forged`, "daemon"},
		`"/tmp/forged""daemon"`:        nil,
		`'/tmp/unterminated daemon`:    nil,
		`"/tmp/bad \q escape" daemon`:  nil,
	}
	for value, want := range cases {
		binary, args, err := parseSystemdCommand(value)
		if want == nil {
			if err == nil {
				t.Errorf("%s: parsed as %q %q, want an error", value, binary, args)
			}
			continue
		}
		if err != nil || binary != want[0] || !slices.Equal(args, want[1:]) {
			t.Errorf("%s: got %q %q %v, want %q", value, binary, args, err, want)
		}
	}
}

// Whatever path Forged writes into the unit, inspection must read back the same command.
func FuzzExecStartRoundTrip(f *testing.F) {
	f.Add("/home/dev/.local/share/forged/bin/forged", "daemon")
	f.Add("/home/a b/100%/for\"ged", "dae mon")
	f.Add("/tmp/\x00\xff'", "")
	f.Fuzz(func(t *testing.T, binary, arg string) {
		if binary == "" {
			return
		}
		gotBinary, gotArgs, err := parseSystemdCommand(formatSystemdExecStart(RuntimeSpec{Binary: binary, Args: []string{arg}}))
		if err != nil || gotBinary != binary || !slices.Equal(gotArgs, []string{arg}) {
			t.Fatalf("%q %q came back as %q %q (%v)", binary, arg, gotBinary, gotArgs, err)
		}
	})
}
