package debuglog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultLogPath = "/tmp/debug-2055fd.log"
	sessionID      = "2055fd"
)

type runIDKey struct{}

var enabled bool
var configuredLogPath = defaultLogPath

func Configure(isEnabled bool, logPath string) {
	enabled = isEnabled
	if logPath != "" {
		configuredLogPath = logPath
		return
	}
	configuredLogPath = defaultLogPath
}

func WithRunID(ctx context.Context, runID string) context.Context {
	if runID == "" {
		return ctx
	}
	return context.WithValue(ctx, runIDKey{}, runID)
}

func RunIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	runID, _ := ctx.Value(runIDKey{}).(string)
	return runID
}

func Log(ctx context.Context, hypothesisID string, location string, message string, data map[string]any) {
	if !enabled {
		return
	}
	payload := map[string]any{
		"sessionId":    sessionID,
		"runId":        RunIDFromContext(ctx),
		"hypothesisId": hypothesisID,
		"location":     location,
		"message":      message,
		"data":         data,
		"timestamp":    time.Now().UnixMilli(),
	}
	appendNDJSON(payload)
}

func appendNDJSON(payload map[string]any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}
	logPath := os.Getenv("DEBUG_LOG_PATH")
	if logPath == "" {
		logPath = configuredLogPath
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(encoded, '\n'))
}
