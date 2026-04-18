package picker

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var (
	ErrUnavailable = errors.New("file picker unavailable")
	ErrCanceled    = errors.New("file picker canceled")
)

func ChooseFile() (string, error) {
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
		script := `Add-Type -AssemblyName System.Windows.Forms;$d=New-Object System.Windows.Forms.OpenFileDialog;$d.Title='Select an import file';if($d.ShowDialog() -eq 'OK'){Write-Output $d.FileName}`
		return runPickerCommand("powershell", "-NoProfile", "-STA", "-Command", script)
	default:
		return "", ErrUnavailable
	}
}

func ChooseSavePath(defaultName string) (string, error) {
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
		script := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms;$d=New-Object System.Windows.Forms.SaveFileDialog;$d.Title='Save Forged export as';$d.FileName=%q;if($d.ShowDialog() -eq 'OK'){Write-Output $d.FileName}`, defaultName)
		return runPickerCommand("powershell", "-NoProfile", "-STA", "-Command", script)
	default:
		return "", ErrUnavailable
	}
}

func runPickerCommand(name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", ErrUnavailable
	}

	cmd := exec.Command(path, args...)
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
