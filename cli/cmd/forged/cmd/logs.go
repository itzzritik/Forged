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
	logFollowInterval = 250 * time.Millisecond
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Follow daemon logs",
	RunE:  runLogsCommand,
}

func runLogsCommand(cmd *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	return followLog(ctx, cmd.OutOrStdout(), config.DefaultPaths().LogFile())
}

func followLog(ctx context.Context, dst io.Writer, path string) error {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	file, err := os.OpenInRoot(dir, name)
	if os.IsNotExist(err) {
		return fmt.Errorf("No log file found at %s", path)
	}
	if err != nil {
		return fmt.Errorf("opening daemon log: %w", err)
	}
	defer func() { _ = file.Close() }()

	contents, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("reading daemon log: %w", err)
	}
	if _, err := io.Copy(dst, bytes.NewReader(lastLogLines(contents, logTailLineCount))); err != nil {
		return fmt.Errorf("writing daemon log: %w", err)
	}

	ticker := time.NewTicker(logFollowInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return fmt.Errorf("following daemon log: %w", ctx.Err())
		case <-ticker.C:
		}

		if _, err := io.Copy(dst, file); err != nil {
			return fmt.Errorf("following daemon log: %w", err)
		}
		offset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("reading daemon log position: %w", err)
		}

		pathFile, err := os.OpenInRoot(dir, name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking daemon log: %w", err)
		}
		pathInfo, err := pathFile.Stat()
		if err != nil {
			_ = pathFile.Close()
			return fmt.Errorf("checking daemon log: %w", err)
		}
		fileInfo, err := file.Stat()
		if err != nil {
			_ = pathFile.Close()
			return fmt.Errorf("checking open daemon log: %w", err)
		}

		if os.SameFile(fileInfo, pathInfo) {
			_ = pathFile.Close()
			if pathInfo.Size() < offset {
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					return fmt.Errorf("resetting truncated daemon log: %w", err)
				}
				if _, err := io.Copy(dst, file); err != nil {
					return fmt.Errorf("following truncated daemon log: %w", err)
				}
			}
			continue
		}

		if _, err := io.Copy(dst, file); err != nil {
			_ = pathFile.Close()
			return fmt.Errorf("finishing rotated daemon log: %w", err)
		}
		_ = file.Close()
		file = pathFile
		if _, err := io.Copy(dst, file); err != nil {
			return fmt.Errorf("following rotated daemon log: %w", err)
		}
	}
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
