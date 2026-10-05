//go:build e2e && windows

// fakesched stands in for schtasks.exe and powershell.exe: the real Task
// Scheduler would start the test daemon in the developer's real profile.
package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"

	"github.com/itzzritik/forged/cli/internal/platform"
	"golang.org/x/sys/windows"
)

func main() {
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0])))
	dir := os.Getenv("FORGED_E2E_SCHED_DIR")
	if dir == "" {
		fmt.Fprintln(os.Stderr, "fakesched: FORGED_E2E_SCHED_DIR unset")
		os.Exit(2)
	}
	switch name {
	case "schtasks":
		os.Exit(schtasks(dir, os.Args[1:]))
	case "powershell":
		os.Exit(taskQuery(dir, strings.Join(os.Args[1:], " ")))
	}
	os.Exit(2)
}

func flag(args []string, name string) (string, bool) {
	for i, a := range args {
		if strings.EqualFold(a, name) {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "/") {
				return args[i+1], true
			}
			return "", true
		}
	}
	return "", false
}

func has(args []string, name string) bool {
	_, ok := flag(args, name)
	return ok
}

func notFound() int {
	fmt.Fprintln(os.Stderr, "ERROR: The system cannot find the file specified.")
	return 1
}

func schtasks(dir string, args []string) int {
	tn, _ := flag(args, "/TN")
	xmlPath := filepath.Join(dir, tn+".xml")
	pidPath := filepath.Join(dir, tn+".pid")
	switch {
	case has(args, "/Create"):
		src, _ := flag(args, "/XML")
		data, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			return 1
		}
		if _, _, err := taskCommand(data); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR: The task XML is malformed:", err)
			return 1
		}
		if err := os.WriteFile(xmlPath, data, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			return 1
		}
		return 0
	case has(args, "/Run"):
		data, err := os.ReadFile(xmlPath)
		if err != nil {
			return notFound()
		}
		if platform.ProcessAlive(readPID(pidPath)) {
			return 0 // MultipleInstancesPolicy IgnoreNew
		}
		command, cmdArgs, err := taskCommand(data)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			return 1
		}
		cmd := exec.Command(command, cmdArgs...)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			return 1
		}
		_ = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600)
		_ = cmd.Process.Release()
		return 0
	case has(args, "/End"):
		if _, err := os.Stat(xmlPath); err != nil {
			return notFound()
		}
		pid := readPID(pidPath)
		if !platform.ProcessAlive(pid) {
			fmt.Fprintf(os.Stderr, "ERROR: The scheduled task %q is not currently running.\n", tn)
			return 1
		}
		terminate(pid)
		_ = os.Remove(pidPath)
		return 0
	case has(args, "/Delete"):
		if _, err := os.Stat(xmlPath); err != nil {
			return notFound()
		}
		_ = os.Remove(xmlPath)
		return 0
	}
	fmt.Fprintln(os.Stderr, "fakesched: unsupported schtasks call", args)
	return 2
}

// taskQuery answers the product's COM task query; anything else fails loudly.
func taskQuery(dir, script string) int {
	task := os.Getenv("FORGED_TASK_NAME")
	if task == "" || !strings.Contains(script, "Schedule.Service") {
		fmt.Fprintln(os.Stderr, "fakesched: unexpected powershell call")
		return 2
	}
	data, err := os.ReadFile(filepath.Join(dir, task+".xml"))
	if err != nil {
		return 3
	}
	command, cmdArgs, err := taskCommand(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	state, pids := 3, []int{}
	if pid := readPID(filepath.Join(dir, task+".pid")); platform.ProcessAlive(pid) {
		state, pids = 4, []int{pid}
	}
	out, _ := json.Marshal(map[string]any{
		"state":   state,
		"actions": []map[string]any{{"type": 0, "path": command, "arguments": strings.Join(cmdArgs, " ")}},
		"pids":    pids,
	})
	os.Stdout.Write(out)
	return 0
}

// Real schtasks rules: UTF-16LE needs its BOM, and 8-bit input is accepted
// only without an XML declaration.
func taskCommand(data []byte) (string, []string, error) {
	var text string
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		units := make([]uint16, (len(data)-2)/2)
		for i := range units {
			units[i] = uint16(data[2+2*i]) | uint16(data[3+2*i])<<8
		}
		text = string(utf16.Decode(units))
	case bytes.HasPrefix(data, []byte("<?xml")):
		return "", nil, fmt.Errorf("(1,40)::ERROR: unable to switch the encoding")
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return "", nil, fmt.Errorf("(1,2)::ERROR: incorrect document syntax")
	default:
		text = string(data)
	}
	var task struct {
		Actions struct {
			Exec []struct {
				Command   string `xml:"Command"`
				Arguments string `xml:"Arguments"`
			} `xml:"Exec"`
		} `xml:"Actions"`
	}
	text = regexp.MustCompile(`encoding="[^"]*"`).ReplaceAllString(text, `encoding="UTF-8"`)
	if err := xml.Unmarshal([]byte(text), &task); err != nil {
		return "", nil, err
	}
	if len(task.Actions.Exec) != 1 {
		return "", nil, fmt.Errorf("expected one Exec action")
	}
	return task.Actions.Exec[0].Command, strings.Fields(task.Actions.Exec[0].Arguments), nil
}

func readPID(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

func terminate(pid int) {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	_ = windows.TerminateProcess(h, 1)
	_, _ = windows.WaitForSingleObject(h, 5000)
}
