package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Status struct {
	PID         int       `json:"pid"`
	Phase       string    `json:"phase"`
	UpdatedAt   time.Time `json:"updated_at"`
	LastSuccess time.Time `json:"last_success,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	Progress    *Progress `json:"progress,omitempty"`
}

func ReadStatus(id string) Status {
	dir, err := Dir(id)
	if err != nil {
		return Status{}
	}
	data, err := os.ReadFile(filepath.Join(dir, "status.json"))
	if err != nil {
		return Status{}
	}
	var s Status
	_ = json.Unmarshal(data, &s)
	return s
}

func acquire(id string) (*os.File, error) {
	dir, err := Dir(id)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "run.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err = tryLock(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("session %s is already running: %w", id, err)
	}
	return f, nil
}

func Running(id string) bool {
	f, err := acquire(id)
	if err != nil {
		return true
	}
	unlock(f)
	f.Close()
	return false
}

// reconcileWatches follows directory entries, never directory symlinks.
func reconcileWatches(w *fsnotify.Watcher, c Config) error {
	wanted := map[string]bool{filepath.Dir(c.LocalDir): true}
	err := filepath.WalkDir(c.LocalDir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(c.LocalDir, name)
		if err != nil {
			return err
		}
		if !c.Relevant(rel) {
			return filepath.SkipDir
		}
		wanted[name] = true
		return nil
	})
	if err != nil {
		return err
	}
	for _, name := range w.WatchList() {
		if wanted[name] {
			delete(wanted, name)
		} else {
			_ = w.Remove(name)
		}
	}
	for name := range wanted {
		if err := w.Add(name); err != nil {
			return err
		}
	}
	return nil
}

type syncFunc func(context.Context) (string, error)
type runOptions struct{ Debounce, MaxDelay, Reconcile, Retry time.Duration }

var defaults = runOptions{750 * time.Millisecond, 3 * time.Second, 30 * time.Second, 5 * time.Second}

func Run(ctx context.Context, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	lock, err := acquire(c.ID)
	if err != nil {
		return err
	}
	defer lock.Close()
	defer unlock(lock)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cleanup, err := controlWorker(c.ID, cancel)
	if err != nil {
		return err
	}
	defer cleanup()
	dir, _ := Dir(c.ID)
	logger := log.New(&rotatingLog{path: filepath.Join(dir, "session.log")}, "", log.LstdFlags)
	t, err := NewTransport(c.Host, c.Auth, c.Password)
	if err != nil {
		logger.Print(err)
		_ = writeJSON(filepath.Join(dir, "status.json"), Status{PID: os.Getpid(), Phase: "stopped", UpdatedAt: time.Now(), LastError: err.Error()})
		return err
	}
	defer t.Close()
	return runLoop(ctx, c, func(ctx context.Context) (string, error) { return t.Sync(ctx, refreshSelections(c), false) }, logger, defaults)
}

func runLoop(ctx context.Context, c Config, syncNow syncFunc, logger *log.Logger, o runOptions) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	dir, _ := Dir(c.ID)
	state := Status{PID: os.Getpid(), Phase: "starting"}
	publish := func(phase string) {
		state.Phase, state.UpdatedAt = phase, time.Now()
		if err := writeJSON(filepath.Join(dir, "status.json"), state); err != nil {
			logger.Printf("status: %v", err)
		}
	}
	publish("starting")
	defer publish("stopped")
	if err := reconcileWatches(w, c); err != nil {
		logger.Printf("watch: %v; periodic reconciliation remains active", err)
	}
	ticker := time.NewTicker(o.Reconcile)
	defer ticker.Stop()
	timer := time.NewTimer(0)
	defer timer.Stop()
	type result struct {
		output string
		err    error
	}
	finished := make(chan result, 1)
	updates := make(chan Progress, 1)
	syncCtx := context.WithValue(ctx, progressKey{}, progressReporter(func(p Progress) {
		select {
		case <-updates:
		default:
		}
		updates <- p
	}))
	progressTick := time.NewTicker(250 * time.Millisecond)
	defer progressTick.Stop()
	progressDirty := false
	consumeProgress := func(p Progress) {
		state.Progress = &p
		progressDirty = true
	}
	running, dirty := false, true
	var firstChange time.Time
	retry := o.Retry
	arm := func(delay time.Duration) {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(delay)
	}
	for {
		select {
		case <-ctx.Done():
			cancel()
			if running {
				<-finished
			}
			select {
			case p := <-updates:
				consumeProgress(p)
			default:
			}
			return nil
		case p := <-updates:
			if running {
				consumeProgress(p)
			}
		case <-progressTick.C:
			if running && progressDirty {
				publish("syncing")
				progressDirty = false
			}
		case event, ok := <-w.Events:
			if !ok {
				return fmt.Errorf("filesystem watcher closed")
			}
			rel, err := filepath.Rel(c.LocalDir, event.Name)
			if err != nil || !c.Relevant(rel) {
				continue
			}
			dirty = true
			if firstChange.IsZero() {
				firstChange = time.Now()
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
				if err := reconcileWatches(w, c); err != nil {
					logger.Printf("watch: %v", err)
				}
			}
			if !running {
				arm(max(0, min(o.Debounce, o.MaxDelay-time.Since(firstChange))))
			}
		case err, ok := <-w.Errors:
			if !ok {
				return fmt.Errorf("filesystem watcher errors channel closed")
			}
			logger.Printf("watch: %v", err)
			dirty = true
			if !running {
				arm(o.Debounce)
			}
		case <-ticker.C:
			if err := reconcileWatches(w, c); err != nil {
				logger.Printf("watch: %v", err)
			}
			dirty = true
			if !running {
				arm(0)
			}
		case <-timer.C:
			if running || !dirty {
				continue
			}
			running, dirty = true, false
			firstChange = time.Time{}
			state.Progress = &Progress{Stage: "connecting", UpdatedAt: time.Now()}
			progressDirty = false
			publish("syncing")
			go func() { out, err := syncNow(syncCtx); finished <- result{out, err} }()
		case result := <-finished:
			select {
			case p := <-updates:
				consumeProgress(p)
			default:
			}
			running = false
			c = refreshSelections(c)
			if err := reconcileWatches(w, c); err != nil {
				logger.Printf("watch: %v", err)
			}
			if c.Password != "" {
				result.output = strings.ReplaceAll(result.output, c.Password, "[redacted]")
			}
			if result.err != nil {
				state.LastError = result.err.Error()
				if c.Password != "" {
					state.LastError = strings.ReplaceAll(state.LastError, c.Password, "[redacted]")
				}
				logger.Print(state.LastError)
				publish("retrying")
				dirty = true
				arm(retry)
				retry = min(60*time.Second, retry*2)
			} else {
				complete := 100
				if state.Progress == nil {
					state.Progress = &Progress{}
				}
				state.Progress.Stage, state.Progress.Percent = "completed", &complete
				state.Progress.Estimated, state.Progress.Rate, state.Progress.ETA = false, "", ""
				state.Progress.UpdatedAt = time.Now()
				state.LastSuccess, state.LastError = time.Now(), ""
				retry = o.Retry
				if strings.TrimSpace(result.output) != "" {
					logger.Print(strings.TrimSpace(result.output))
				}
				publish("watching")
				if dirty {
					arm(o.Debounce)
				}
			}
		}
	}
}

type rotatingLog struct{ path string }

func (l *rotatingLog) Write(p []byte) (int, error) {
	if info, err := os.Stat(l.path); err == nil && info.Size() > 2*1024*1024 {
		_ = os.Remove(l.path + ".1")
		_ = os.Rename(l.path, l.path+".1")
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return f.Write(p)
}

// Only a completed relocation changes a running worker's selected paths.
func refreshSelections(c Config) Config {
	saved, err := Load(c.ID)
	if err == nil && saved.Host == c.Host && saved.LocalDir == c.LocalDir && saved.RemoteParent == c.RemoteParent {
		c.Selections = saved.Selections
		c.SelectionMode = saved.SelectionMode
	}
	return c
}
