package webui

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func Dispatch(args []string) (bool, int) {
	var filtered []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--lang" && i+1 < len(args) {
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--lang=") {
			continue
		}
		filtered = append(filtered, args[i])
	}
	if len(filtered) == 0 || filtered[0] != "ui" {
		return false, 0
	}
	set := flag.NewFlagSet("xsync ui", flag.ContinueOnError)
	port := set.Int("port", 8787, "Cổng giao diện local; dùng 0 để chọn cổng trống")
	noOpen := set.Bool("no-open", false, "Không tự mở trình duyệt")
	set.Usage = func() {
		fmt.Fprintln(set.Output(), "Cách dùng: xsync ui [--port 8787] [--no-open]\nMở giao diện web local để chọn thư mục và quản lý phiên đồng bộ.")
		set.PrintDefaults()
	}
	if err := set.Parse(filtered[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return true, 0
		}
		return true, 1
	}
	if set.NArg() != 0 || *port < 0 || *port > 65535 {
		fmt.Fprintln(os.Stderr, "Cổng hoặc tham số giao diện không hợp lệ.")
		return true, 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := Serve(ctx, *port, !*noOpen); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
	return true, 0
}
