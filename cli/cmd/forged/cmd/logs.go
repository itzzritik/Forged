package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/spf13/cobra"
)

const (
	logTailLineCount  = 10
	logTailByteLimit  = 64 * 1024
	logFollowInterval = 250 * time.Millisecond
)

var logsCmd = &cobra.Command{
	Use:       "logs [daemon|stderr|tui]",
	Short:     "Follow daemon, stderr, or TUI logs",
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"daemon", "stderr", "tui"},
	RunE:      runLogsCommand,
}

func runLogsCommand(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	paths := config.DefaultPaths()
	path := paths.LogFile()
	if len(args) > 0 {
		switch args[0] {
		case "daemon":
		case "stderr":
			path = paths.DaemonStderrLogFile()
		case "tui":
			path = paths.TUILogFile()
		default:
			return fmt.Errorf("Unknown log source %q (use daemon, stderr, or tui)", args[0])
		}
	}

	return followLog(ctx, cmd.OutOrStdout(), path)
}

func followLog(ctx context.Context, dst io.Writer, path string) error {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	file, err := os.OpenInRoot(dir, name)
	if os.IsNotExist(err) {
		return fmt.Errorf("No log file found at %s", path)
	}
	if err != nil {
		return fmt.Errorf("opening log: %w", err)
	}
	defer func() { _ = file.Close() }()

	contents, err := readLogTail(file)
	if err != nil {
		return fmt.Errorf("reading log: %w", err)
	}
	if _, err := io.Copy(dst, bytes.NewReader(lastLogLines(contents, logTailLineCount))); err != nil {
		return fmt.Errorf("writing log: %w", err)
	}

	ticker := time.NewTicker(logFollowInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return fmt.Errorf("following log: %w", ctx.Err())
		case <-ticker.C:
		}

		if _, err := io.Copy(dst, file); err != nil {
			return fmt.Errorf("following log: %w", err)
		}
		offset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("reading log position: %w", err)
		}

		pathFile, err := os.OpenInRoot(dir, name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking log: %w", err)
		}
		pathInfo, err := pathFile.Stat()
		if err != nil {
			_ = pathFile.Close()
			return fmt.Errorf("checking log: %w", err)
		}
		fileInfo, err := file.Stat()
		if err != nil {
			_ = pathFile.Close()
			return fmt.Errorf("checking open log: %w", err)
		}

		if os.SameFile(fileInfo, pathInfo) {
			_ = pathFile.Close()
			if pathInfo.Size() < offset {
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					return fmt.Errorf("resetting truncated log: %w", err)
				}
				if _, err := io.Copy(dst, file); err != nil {
					return fmt.Errorf("following truncated log: %w", err)
				}
			}
			continue
		}

		if _, err := io.Copy(dst, file); err != nil {
			_ = pathFile.Close()
			return fmt.Errorf("finishing rotated log: %w", err)
		}
		_ = file.Close()
		file = pathFile
		if _, err := io.Copy(dst, file); err != nil {
			return fmt.Errorf("following rotated log: %w", err)
		}
	}
}

func readLogTail(file *os.File) ([]byte, error) {
	end, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	size := end
	if size > logTailByteLimit {
		size = logTailByteLimit
	}
	if size == 0 {
		return nil, nil
	}
	start := end - size
	partialLine := false
	if start > 0 {
		var previous [1]byte
		if _, err := file.ReadAt(previous[:], start-1); err == nil {
			partialLine = previous[0] != '\n'
		} else if !errors.Is(err, io.EOF) {
			return nil, err
		}
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(io.LimitReader(file, size))
	if err != nil {
		return nil, err
	}
	if partialLine {
		index := bytes.IndexByte(contents, '\n')
		if index < 0 {
			return nil, nil
		}
		contents = contents[index+1:]
	}
	return contents, nil
}

func lastLogLines(contents []byte, count int) []byte {
	if count <= 0 || len(contents) == 0 {
		return nil
	}

	end := len(contents)
	if contents[end-1] == '\n' {
		end--
	}
	for found := 0; end > 0; found++ {
		index := bytes.LastIndexByte(contents[:end], '\n')
		if index < 0 {
			return contents
		}
		if found == count-1 {
			return contents[index+1:]
		}
		end = index
	}
	return contents
}
