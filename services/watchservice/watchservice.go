package watchservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Watcher struct {
	Root     string
	Interval time.Duration
	Debounce time.Duration
}

type watchedFile struct {
	modTime time.Time
	size    int64
	hash    string
	hasHash bool
}

func (w Watcher) Watch(ctx context.Context) <-chan []string {
	out := make(chan []string, 1)
	go func() {
		defer close(out)
		previous, _ := scanWatchedFiles(w.Root)
		ticker := time.NewTicker(w.Interval)
		defer ticker.Stop()

		pending := map[string]bool{}
		var timer *time.Timer
		var timerC <-chan time.Time

		for {
			select {
			case <-ctx.Done():
				if timer != nil {
					timer.Stop()
				}
				return
			case <-ticker.C:
				next, err := scanWatchedFiles(w.Root)
				if err != nil {
					continue
				}
				changes := diffSnapshots(previous, next)
				previous = next
				if len(changes) == 0 {
					continue
				}
				for _, path := range changes {
					pending[path] = true
				}
				if timer == nil {
					timer = time.NewTimer(w.Debounce)
				} else {
					resetTimer(timer, w.Debounce)
				}
				timerC = timer.C
			case <-timerC:
				changes := sortedPending(pending)
				pending = map[string]bool{}
				timerC = nil
				select {
				case out <- changes:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

func scanWatchedFiles(root string) (map[string]watchedFile, error) {
	files := map[string]watchedFile{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if entry.IsDir() {
			if skipWatchDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !WatchPath(rel) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		file := watchedFile{
			modTime: info.ModTime(),
			size:    info.Size(),
		}
		if shouldHashWatchedFile(rel) {
			hash, err := fileHash(path)
			if err == nil {
				file.hash = hash
				file.hasHash = true
			}
		}
		files[rel] = file
		return nil
	})
	return files, err
}

func diffSnapshots(previous map[string]watchedFile, next map[string]watchedFile) []string {
	changed := map[string]bool{}
	for path, nextInfo := range next {
		previousInfo, ok := previous[path]
		if !ok || !sameWatchedFile(previousInfo, nextInfo) {
			changed[path] = true
		}
	}
	for path := range previous {
		if _, ok := next[path]; !ok {
			changed[path] = true
		}
	}
	return sortedPending(changed)
}

func sameWatchedFile(previous, next watchedFile) bool {
	if previous.size != next.size {
		return false
	}
	if previous.hasHash || next.hasHash {
		return previous.hasHash && next.hasHash && previous.hash == next.hash
	}
	return previous.modTime.Equal(next.modTime)
}

func WatchPath(path string) bool {
	base := filepath.Base(path)
	if skipWatchFile(base) {
		return false
	}
	if isTopLevelWatchedFile(path) {
		return true
	}
	ext := filepath.Ext(path)
	if ext == ".go" {
		return !strings.HasSuffix(path, "_test.go")
	}
	first := firstPathPart(path)
	switch first {
	case "app", "cmd", "init", "internal", "lib", "pkg", "public", "services", "styles", "views":
		return true
	default:
		return false
	}
}

func isTopLevelWatchedFile(path string) bool {
	if strings.Contains(path, "/") {
		return false
	}
	switch path {
	case "go.mod", "go.sum", "js.toml", "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb", "tailwind.config.js":
		return true
	default:
		return false
	}
}

func shouldHashWatchedFile(path string) bool {
	return isTopLevelJavaScriptPackageInput(path)
}

func isTopLevelJavaScriptPackageInput(path string) bool {
	if strings.Contains(path, "/") {
		return false
	}
	switch path {
	case "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb":
		return true
	default:
		return false
	}
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func skipWatchDir(name string) bool {
	switch name {
	case ".git", ".lazy", "bin", "dist", "node_modules", "tmp", "vendor":
		return true
	default:
		return false
	}
}

func skipWatchFile(name string) bool {
	if name == ".DS_Store" {
		return true
	}
	if strings.HasPrefix(name, ".#") {
		return true
	}
	for _, suffix := range []string{"~", ".swp", ".swo", ".tmp"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func firstPathPart(path string) string {
	if index := strings.IndexByte(path, '/'); index >= 0 {
		return path[:index]
	}
	return path
}

func sortedPending(pending map[string]bool) []string {
	paths := make([]string, 0, len(pending))
	for path := range pending {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}
