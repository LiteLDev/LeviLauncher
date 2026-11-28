package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const fileOpenEvent = "content-files.open"

var FileOpens = &FileOpenService{}

type FileOpenService struct {
	mu      sync.Mutex
	pending []string
}

// ContentFileArgs recognises shell file activations before CLI dispatch.
func ContentFileArgs(args []string) []string {
	var files []string
	for _, arg := range args {
		switch strings.ToLower(filepath.Ext(arg)) {
		case ".mcpack", ".mcaddon", ".mcworld", ".mcmod":
			if strings.HasPrefix(arg, "-") {
				continue
			}
			path, err := filepath.Abs(arg)
			if err != nil {
				continue
			}
			if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
				files = append(files, path)
			}
		}
	}
	return files
}

func QueueContentFiles(args []string) {
	files := ContentFileArgs(args)
	if len(files) == 0 {
		return
	}
	FileOpens.mu.Lock()
	FileOpens.pending = append(FileOpens.pending, files...)
	FileOpens.mu.Unlock()
	if a := application.Get(); a != nil {
		go launch.RestoreLauncherWindow()
		a.Event.Emit(fileOpenEvent)
	}
}

func (s *FileOpenService) TakeFiles() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) == 0 {
		return nil
	}
	isMod := strings.EqualFold(filepath.Ext(s.pending[0]), ".mcmod")
	end := 1
	for end < len(s.pending) && strings.EqualFold(filepath.Ext(s.pending[end]), ".mcmod") == isMod {
		end++
	}
	files := s.pending[:end:end]
	s.pending = s.pending[end:]
	return files
}
