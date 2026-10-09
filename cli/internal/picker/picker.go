package picker

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/itzzritik/forged/cli/internal/platform"
)

var (
	ErrUnavailable = errors.New("File picker unavailable")
	ErrCanceled    = errors.New("File picker canceled")
)

func Available() bool {
	switch {
	case platform.OverSSH():
		return false
	case runtime.GOOS == "linux":
		return platform.HasDisplay()
	}
	return runtime.GOOS == "darwin" || runtime.GOOS == "windows"
}

func ChooseFile() (string, error) {
	if !Available() {
		return "", ErrUnavailable
	}
	switch runtime.GOOS {
	case "darwin":
		return runPickerCommand(
			"osascript",
			"-e", `POSIX path of (choose file with prompt "Select an import file")`,
		)
	case "linux":
		if selection, err := runPickerCommand("zenity", "--file-selection", "--title=Select an import file"); err == nil || !errors.Is(err, ErrUnavailable) {
			return selection, err
		}
		return runPickerCommand("kdialog", "--getopenfilename")
	case "windows":
		return runWindowsPicker(`$d=New-Object System.Windows.Forms.OpenFileDialog;$d.Title='Select an import file'`)
	default:
		return "", ErrUnavailable
	}
}

func ChooseSavePath(defaultName string) (string, error) {
	if !Available() {
		return "", ErrUnavailable
	}
	switch runtime.GOOS {
	case "darwin":
		return runPickerCommand(
			"osascript",
			"-e", fmt.Sprintf(`POSIX path of (choose file name with prompt "Save Forged export as" default name %q)`, defaultName),
		)
	case "linux":
		if selection, err := runPickerCommand("zenity", "--file-selection", "--save", "--confirm-overwrite", "--filename="+defaultName, "--title=Save Forged export as"); err == nil || !errors.Is(err, ErrUnavailable) {
			return selection, err
		}
		return runPickerCommand("kdialog", "--getsavefilename", defaultName)
	case "windows":
		return runWindowsPicker(`$d=New-Object System.Windows.Forms.SaveFileDialog;$d.Title='Save Forged export as';$d.FileName=$env:FORGED_PICKER_NAME`,
			"FORGED_PICKER_NAME="+defaultName)
	default:
		return "", ErrUnavailable
	}
}

// The name travels in env (no quoting), a topmost owner keeps the dialog in
// front, and base64 survives the console code page.
func runWindowsPicker(dialog string, env ...string) (string, error) {
	script := `Add-Type -AssemblyName System.Windows.Forms;` + dialog +
		`;$owner=New-Object System.Windows.Forms.Form -Property @{TopMost=$true;ShowInTaskbar=$false}` +
		`;try{if($d.ShowDialog($owner) -eq 'OK'){[Console]::Out.Write([Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($d.FileName)))}}finally{$owner.Dispose()}`
	encoded, err := runPickerCommandEnv(env, "powershell", "-NoProfile", "-STA", "-EncodedCommand", platform.PowerShellEncodedCommand(script))
	if err != nil {
		return "", err
	}
	path, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding file picker result: %w", err)
	}
	return string(path), nil
}

func runPickerCommand(name string, args ...string) (string, error) {
	return runPickerCommandEnv(nil, name, args...)
}

func runPickerCommandEnv(env []string, name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", ErrUnavailable
	}

	cmd := exec.Command(path, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", ErrCanceled
		}
		return "", err
	}

	selection := strings.TrimSpace(string(out))
	if selection == "" {
		return "", ErrCanceled
	}
	return selection, nil
}
