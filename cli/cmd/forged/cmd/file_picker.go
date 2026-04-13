package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func terminalIsInteractive() bool {
	stdinInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	stdoutInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stdinInfo.Mode()&os.ModeCharDevice) != 0 && (stdoutInfo.Mode()&os.ModeCharDevice) != 0
}

func clearTerminal() {
	if !terminalIsInteractive() {
		return
	}
	fmt.Print("\033[2J\033[H")
}

func chooseFileWithPicker() (string, bool) {
	switch runtime.GOOS {
	case "darwin":
		return macOSPickFile()
	case "linux":
		return linuxPickFile()
	case "windows":
		return windowsPickFile()
	default:
		return "", false
	}
}

func runPickerCommand(name string, args ...string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	out, err := exec.Command(path, args...).Output()
	if err != nil {
		return "", false
	}
	selection := strings.TrimSpace(string(out))
	if selection == "" {
		return "", false
	}
	return selection, true
}

func macOSPickFile() (string, bool) {
	return runPickerCommand(
		"osascript",
		"-e", `POSIX path of (choose file with prompt "Select an import file")`,
	)
}

func linuxPickFile() (string, bool) {
	if selection, ok := runPickerCommand("zenity", "--file-selection", "--title=Select an import file"); ok {
		return selection, true
	}
	return runPickerCommand("kdialog", "--getopenfilename")
}

func windowsPickFile() (string, bool) {
	script := `Add-Type -AssemblyName System.Windows.Forms;$d=New-Object System.Windows.Forms.OpenFileDialog;$d.Title='Select an import file';if($d.ShowDialog() -eq 'OK'){Write-Output $d.FileName}`
	return runPickerCommand("powershell", "-NoProfile", "-STA", "-Command", script)
}
