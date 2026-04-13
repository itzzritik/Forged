package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/term"
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

func printStepSeparator() {
	if !terminalIsInteractive() {
		return
	}

	const (
		indentWidth       = 2
		fallbackLineWidth = 56
		minLineWidth      = 48
		maxLineWidth      = 88
	)

	lineWidth := fallbackLineWidth
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > indentWidth {
		lineWidth = width - indentWidth
		if lineWidth < minLineWidth {
			lineWidth = minLineWidth
		}
		if lineWidth > maxLineWidth {
			lineWidth = maxLineWidth
		}
	}

	fmt.Println()
	fmt.Printf("  \033[2m%s\033[0m\n", strings.Repeat("─", lineWidth))
	fmt.Println()
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

func chooseSavePathWithPicker(defaultName string) (string, bool) {
	switch runtime.GOOS {
	case "darwin":
		return macOSSaveFile(defaultName)
	case "linux":
		return linuxSaveFile(defaultName)
	case "windows":
		return windowsSaveFile(defaultName)
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

func macOSSaveFile(defaultName string) (string, bool) {
	return runPickerCommand(
		"osascript",
		"-e", fmt.Sprintf(`POSIX path of (choose file name with prompt "Save Forged export as" default name %q)`, defaultName),
	)
}

func linuxSaveFile(defaultName string) (string, bool) {
	if selection, ok := runPickerCommand("zenity", "--file-selection", "--save", "--confirm-overwrite", "--filename="+defaultName, "--title=Save Forged export as"); ok {
		return selection, true
	}
	return runPickerCommand("kdialog", "--getsavefilename", defaultName)
}

func windowsPickFile() (string, bool) {
	script := `Add-Type -AssemblyName System.Windows.Forms;$d=New-Object System.Windows.Forms.OpenFileDialog;$d.Title='Select an import file';if($d.ShowDialog() -eq 'OK'){Write-Output $d.FileName}`
	return runPickerCommand("powershell", "-NoProfile", "-STA", "-Command", script)
}

func windowsSaveFile(defaultName string) (string, bool) {
	script := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms;$d=New-Object System.Windows.Forms.SaveFileDialog;$d.Title='Save Forged export as';$d.FileName=%q;if($d.ShowDialog() -eq 'OK'){Write-Output $d.FileName}`, defaultName)
	return runPickerCommand("powershell", "-NoProfile", "-STA", "-Command", script)
}

func promptForSavePath(defaultName string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("  Save path [%s]: ", defaultName)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultName, nil
	}
	return line, nil
}
