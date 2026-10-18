package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"golang.org/x/term"
	"xsync/internal/i18n"
	"xsync/internal/migration"
	"xsync/internal/session"
	"xsync/internal/sessioncli"
	"xsync/internal/tui"
)

// Release builds set Version through the linker; source builds identify themselves as dev.
var Version = "dev"

func versionLabel() string {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" && len(setting.Value) >= 7 {
					return "xsync dev (" + setting.Value[:7] + ")"
				}
			}
		}
	}
	return "xsync " + Version
}

func main() { os.Exit(run(os.Args[1:])) }

func help() string {
	return tui.Text(`xsync · Đồng bộ thư mục

  xsync             Mở giao diện terminal tương tác
  xsync --help      Xem hướng dẫn
  xsync --version   Xem phiên bản

Trong giao diện, dùng ↑/↓ và Enter để quản lý phiên, máy chủ,
mật khẩu, SSH key, giao diện web và cài đặt. Esc để quay lại.
Chọn Cài đặt để đổi ngôn ngữ và xem nơi lưu dữ liệu.`, `xsync · Folder synchronization

  xsync             Open the interactive terminal interface
  xsync --help      Show help
  xsync --version   Show version

Use ↑/↓ and Enter to manage sessions, servers, passwords, SSH keys,
the web interface and settings. Press Esc to go back.
Open Settings to change the language and view storage locations.`)
}

func workerInvocation(args []string) (string, bool) {
	if len(args) == 2 && args[0] == session.WorkerCommand {
		return args[1], true
	}
	// Keep service definitions created by previous versions working.
	if len(args) == 3 && args[0] == "session" && args[1] == "run" {
		return args[2], true
	}
	return "", false
}

func migrateProject() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	i18n.LoadSavedLanguage(filepath.Join(dir, "xsync.ini"))
	backup, err := migration.Legacy(dir)
	if err != nil {
		return err
	}
	if backup != "" {
		fmt.Fprintln(os.Stderr, tui.Text("Đã chuyển cấu hình cũ; bản sao lưu: ", "Legacy configuration migrated; backup: ")+backup)
	}
	return nil
}

func run(args []string) int {
	// Workers never enter the TUI or inspect a project's legacy configuration.
	if id, ok := workerInvocation(args); ok {
		if err := sessioncli.RunWorker(id); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	i18n.LoadSavedLanguage("")
	if len(args) == 1 {
		switch args[0] {
		case "--help", "-h":
			fmt.Println(help())
			return 0
		case "--version", "-v":
			fmt.Println(versionLabel())
			return 0
		}
	}
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, tui.Text("Hãy chạy xsync và chọn thao tác trong giao diện terminal.", "Run xsync and choose an action in the terminal interface."))
		fmt.Fprintln(os.Stderr, help())
		return 1
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintln(os.Stderr, tui.Text("Cần terminal tương tác để mở xsync.", "An interactive terminal is required to open xsync."))
		fmt.Fprintln(os.Stderr, help())
		return 1
	}
	if err := migrateProject(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := workspace(); err != nil && !errors.Is(err, tui.ErrCancelled) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
