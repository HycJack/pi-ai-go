package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"code-artisan/internal/paths"
	"code-artisan/internal/processutil"
)

type Runner struct {
	mu       sync.Mutex
	running  bool
	stopping bool
	cmd      *exec.Cmd
}

type RunRequest struct {
	OnError  func(error)
	OnFinish func()
	OnStart  func()
	OnStop   func()
}

type RunError struct {
	Type      string
	Traceback string
	Err       error
}

func (e *RunError) Error() string {
	if e.Traceback != "" {
		return e.Type + "\n" + e.Traceback
	}
	if e.Err != nil {
		return e.Type + ": " + e.Err.Error()
	}
	return e.Type
}

func NewRunner() *Runner {
	return &Runner{}
}

func (r *Runner) Run(scriptCode string, req RunRequest) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return errors.New("已有 Python 脚本正在运行")
	}

	python, args, err := resolvePythonCommand()
	if err != nil {
		r.mu.Unlock()
		return err
	}

	scriptsDir, err := paths.ScriptsDir()
	if err != nil {
		r.mu.Unlock()
		return err
	}

	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		r.mu.Unlock()
		return err
	}

	scriptPath := filepath.Join(scriptsDir, "gen_plot.py")
	if err := os.WriteFile(scriptPath, []byte(scriptCode), 0o644); err != nil {
		r.mu.Unlock()
		return err
	}

	cmd := exec.Command(python, append(args, scriptPath)...)
	cmd.Dir = scriptsDir
	cmd.Env = append(os.Environ(), "MPLBACKEND=Qt5Agg")
	cmd.SysProcAttr = processutil.WithoutConsoleWindow()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	r.cmd = cmd
	r.running = true
	r.mu.Unlock()

	if err := cmd.Start(); err != nil {
		r.finish()
		return err
	}

	if req.OnStart != nil {
		req.OnStart()
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				if req.OnError != nil {
					req.OnError(fmt.Errorf("[panic] recovered: %v", r))
				}
				// Print to stderr since log system may not be available
				fmt.Fprintf(os.Stderr, "[runner panic] %v\n%s\n", r, buf[:n])
			}
		}()
		waitErr := cmd.Wait()
		output := strings.TrimSpace(stderr.String())
		stopped := r.finish()

		if stopped {
			if req.OnStop != nil {
				req.OnStop()
			}
			return
		}

		if waitErr != nil && req.OnError != nil {
			req.OnError(&RunError{
				Type:      detectPythonErrorType(output, waitErr),
				Traceback: tail(output, 15),
				Err:       waitErr,
			})
		}

		if waitErr == nil && req.OnFinish != nil {
			req.OnFinish()
		}
	}()

	return nil
}

func (r *Runner) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd != nil && r.cmd.Process != nil {
		_ = killProcessTree(r.cmd.Process.Pid)
	}
}

func (r *Runner) Stop() (bool, error) {
	r.mu.Lock()
	if !r.running || r.cmd == nil || r.cmd.Process == nil {
		r.mu.Unlock()
		return false, nil
	}

	pid := r.cmd.Process.Pid
	r.stopping = true
	r.mu.Unlock()

	if err := killProcessTree(pid); err != nil {
		r.mu.Lock()
		r.stopping = false
		r.mu.Unlock()
		return false, err
	}

	return true, nil
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) finish() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	wasStopping := r.stopping
	r.running = false
	r.stopping = false
	r.cmd = nil
	return wasStopping
}

func resolvePythonCommand() (string, []string, error) {
	runtimeDir, err := paths.RuntimeDir()
	if err != nil {
		return "", nil, err
	}

	pythonPath := filepath.Join(runtimeDir, "python.exe")
	if _, err := os.Stat(pythonPath); err == nil {
		return pythonPath, nil, nil
	}

	pythonwPath := filepath.Join(runtimeDir, "pythonw.exe")
	if _, err := os.Stat(pythonwPath); err == nil {
		return pythonwPath, nil, nil
	}

	// Fallback: try system Python
	if path, err := exec.LookPath("python"); err == nil {
		return path, nil, nil
	}
	if path, err := exec.LookPath("python3"); err == nil {
		return path, nil, nil
	}

	return "", nil, errors.New("嵌入式 Python 未找到，请确保 runtime/ 目录存在")
}

func killProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
		cmd.SysProcAttr = processutil.WithoutConsoleWindow()
		return cmd.Run()
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

func detectPythonErrorType(stderr string, fallback error) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if index := strings.Index(line, ":"); index > 0 {
			candidate := strings.TrimSpace(line[:index])
			if strings.HasSuffix(candidate, "Error") || strings.HasSuffix(candidate, "Exception") {
				return candidate
			}
		}
	}
	if fallback != nil {
		return fallback.Error()
	}
	return "PythonError"
}

func tail(input string, lines int) string {
	if input == "" {
		return ""
	}
	parts := strings.Split(input, "\n")
	if len(parts) <= lines {
		return input
	}
	return strings.Join(parts[len(parts)-lines:], "\n")
}
