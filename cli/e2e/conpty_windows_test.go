//go:build e2e && windows

package e2e

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ptyProcess runs a console program under a pseudo console so the TUI sees
// a real terminal and tests can read what it rendered.
type ptyProcess struct {
	console windows.Handle
	process windows.Handle
	input   *os.File
	output  *os.File
	screen  *screen
	done    chan struct{}
	exit    uint32
}

func startPTY(argv []string, env []string, dir string, cols, rows int) (*ptyProcess, error) {
	var inRead, inWrite, outRead, outWrite windows.Handle
	if err := windows.CreatePipe(&inRead, &inWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("creating input pipe: %w", err)
	}
	if err := windows.CreatePipe(&outRead, &outWrite, nil, 0); err != nil {
		windows.CloseHandle(inRead)
		windows.CloseHandle(inWrite)
		return nil, fmt.Errorf("creating output pipe: %w", err)
	}

	var console windows.Handle
	size := windows.Coord{X: int16(cols), Y: int16(rows)}
	if err := windows.CreatePseudoConsole(size, inRead, outWrite, 0, &console); err != nil {
		for _, h := range []windows.Handle{inRead, inWrite, outRead, outWrite} {
			windows.CloseHandle(h)
		}
		return nil, fmt.Errorf("creating pseudo console: %w", err)
	}
	// The pseudo console holds its own copies; ours would keep output open.
	windows.CloseHandle(inRead)
	windows.CloseHandle(outWrite)

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		windows.ClosePseudoConsole(console)
		return nil, fmt.Errorf("allocating attribute list: %w", err)
	}
	defer attrs.Delete()
	// The attribute value is the HPCON itself, not a pointer to it.
	hpc := *(*unsafe.Pointer)(unsafe.Pointer(&console))
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, hpc, unsafe.Sizeof(console)); err != nil {
		windows.ClosePseudoConsole(console)
		return nil, fmt.Errorf("attaching pseudo console: %w", err)
	}

	si := &windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(*si))
	// Null std handles, or the child inherits the test runner's pipes.
	si.Flags = windows.STARTF_USESTDHANDLES
	cmdline, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(argv))
	if err != nil {
		windows.ClosePseudoConsole(console)
		return nil, err
	}
	var dirPtr *uint16
	if dir != "" {
		if dirPtr, err = windows.UTF16PtrFromString(dir); err != nil {
			windows.ClosePseudoConsole(console)
			return nil, err
		}
	}
	envBlock := environmentBlock(env)
	var pi windows.ProcessInformation
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(nil, cmdline, nil, nil, false, flags, &envBlock[0], dirPtr, &si.StartupInfo, &pi); err != nil {
		windows.ClosePseudoConsole(console)
		return nil, fmt.Errorf("starting %s: %w", argv[0], err)
	}
	windows.CloseHandle(pi.Thread)

	p := &ptyProcess{
		console: console,
		process: pi.Process,
		input:   os.NewFile(uintptr(inWrite), "pty-in"),
		output:  os.NewFile(uintptr(outRead), "pty-out"),
		done:    make(chan struct{}),
	}
	p.screen = newScreen(cols, rows, func(b []byte) { _, _ = p.input.Write(b) })
	go p.readLoop()
	go p.waitLoop()
	return p, nil
}

func (p *ptyProcess) readLoop() {
	buf := make([]byte, 32*1024)
	for {
		n, err := p.output.Read(buf)
		if n > 0 {
			p.screen.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (p *ptyProcess) waitLoop() {
	_, _ = windows.WaitForSingleObject(p.process, windows.INFINITE)
	_ = windows.GetExitCodeProcess(p.process, &p.exit)
	// Closing the console flushes the final output and ends readLoop.
	windows.ClosePseudoConsole(p.console)
	close(p.done)
}

func (p *ptyProcess) Send(s string) error {
	_, err := p.input.Write([]byte(s))
	return err
}

// Type sends one key at a time so the TUI sees discrete key events.
func (p *ptyProcess) Type(s string) error {
	for _, r := range s {
		if err := p.Send(string(r)); err != nil {
			return err
		}
		time.Sleep(15 * time.Millisecond)
	}
	return nil
}

func (p *ptyProcess) Text() string { return p.screen.Text() }

func (p *ptyProcess) WaitFor(timeout time.Duration, needles ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		text := p.Text()
		if containsAll(text, needles) {
			return text, nil
		}
		select {
		case <-p.done:
			if text = p.Text(); containsAll(text, needles) {
				return text, nil
			}
			return text, fmt.Errorf("process exited (code %d) before %q appeared", p.exit, needles)
		default:
		}
		if time.Now().After(deadline) {
			return text, fmt.Errorf("timed out after %s waiting for %q", timeout, needles)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func containsAll(text string, needles []string) bool {
	for _, n := range needles {
		if !strings.Contains(text, n) {
			return false
		}
	}
	return true
}

func (p *ptyProcess) Wait(timeout time.Duration) (uint32, error) {
	select {
	case <-p.done:
		return p.exit, nil
	case <-time.After(timeout):
		return 0, errors.New("timed out waiting for process exit")
	}
}

func (p *ptyProcess) Kill() {
	select {
	case <-p.done:
	default:
		_ = windows.TerminateProcess(p.process, 1)
		<-p.done
	}
	windows.CloseHandle(p.process)
	p.input.Close()
	p.output.Close()
}

func environmentBlock(env []string) []uint16 {
	sorted := append([]string(nil), env...)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToUpper(sorted[i]) < strings.ToUpper(sorted[j])
	})
	var block []uint16
	for _, kv := range sorted {
		block = append(block, utf16.Encode([]rune(kv))...)
		block = append(block, 0)
	}
	return append(block, 0)
}
