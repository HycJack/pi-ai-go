package env

import (
	"os"

	"code-artisan/internal/paths"
)

type Manager struct{}

type Progress struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
	Percent int    `json:"percent"`
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) EnsureReady(onProgress func(Progress)) error {
	scriptsDir, err := paths.ScriptsDir()
	if err != nil {
		return err
	}

	reportProgress(onProgress, Progress{
		Stage:   "preparing",
		Message: "正在准备脚本目录",
		Percent: 6,
	})

	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		return err
	}

	return m.ensureRuntimeExtracted(onProgress)
}

func (m *Manager) Status() (Status, error) {
	return EvaluateStatus(DefaultRequirements())
}

func (m *Manager) Rebuild(onProgress func(Progress)) error {
	archivePath, err := paths.RuntimeArchivePath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(archivePath); err != nil {
		return err
	}

	runtimeDir, err := paths.RuntimeDir()
	if err != nil {
		return err
	}

	reportProgress(onProgress, Progress{
		Stage:   "rebuilding",
		Message: "正在重建运行环境",
		Percent: 8,
	})

	if err := os.RemoveAll(runtimeDir); err != nil {
		return err
	}

	return m.ensureRuntimeExtracted(onProgress)
}

func reportProgress(onProgress func(Progress), progress Progress) {
	if onProgress == nil {
		return
	}
	onProgress(progress)
}
