package core

import (
	"encoding/json"
	"io"
	"noyo/core/config"
	"noyo/core/store"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
)

// handleGetLogConfig returns the current log configuration
func (s *Server) handleGetLogConfig(r *ghttp.Request) {
	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": s.Config.Log,
	})
}

// handleUpdateLogConfig updates the log configuration
func (s *Server) handleUpdateLogConfig(r *ghttp.Request) {
	var req config.LogConfig
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	// Update config in memory
	s.Config.Log = req

	// Save to database
	if err := store.SaveGlobalConfig(s.Config); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to save config: " + err.Error()})
		return
	}

	// Note: We don't dynamically reload zap.Logger here because it's passed by value everywhere.
	// We will inform the user that a restart is required, or we could just accept that some parts won't update until restart.
	// Actually, lumberjack allows modifying its fields dynamically, but we'll keep it simple.

	r.Response.WriteJson(g.Map{
		"code":    0,
		"message": "Log configuration updated. Some changes may require restarting the gateway.",
	})
}

// handleListLogFiles returns a list of historical log files
func (s *Server) handleListLogFiles(r *ghttp.Request) {
	dir := s.Config.Log.Dir
	if dir == "" {
		dir = "./data/logs"
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			r.Response.WriteJson(g.Map{"code": 0, "data": []interface{}{}})
			return
		}
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	var result []map[string]interface{}
	for _, f := range files {
		if f.IsDir() || !strings.HasPrefix(f.Name(), "noyo.log") {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"name": f.Name(),
			"size": info.Size(),
			"time": info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	// Sort by modification time descending
	sort.Slice(result, func(i, j int) bool {
		return result[i]["time"].(string) > result[j]["time"].(string)
	})

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": result,
	})
}

// handleTailLog reads the last N lines of the current log file
func (s *Server) handleTailLog(r *ghttp.Request) {
	lines := r.Get("lines", 100).Int()
	if lines <= 0 {
		lines = 100
	}

	dir := s.Config.Log.Dir
	if dir == "" {
		dir = "./data/logs"
	}
	path := filepath.Join(dir, "noyo.log")

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			r.Response.WriteJson(g.Map{"code": 0, "data": []interface{}{}})
			return
		}
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	var result []map[string]interface{}

	// Read at most 128KB from the end to find the last N lines
	readSize := int64(131072) // 128KB
	if stat.Size() < readSize {
		readSize = stat.Size()
	}
	offset := stat.Size() - readSize

	buf := make([]byte, readSize)
	_, err = file.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	parts := strings.Split(string(buf), "\n")

	// Parse lines as JSON
	for _, line := range parts {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err == nil {
			msg := map[string]interface{}{
				"level":   raw["level"],
				"time":    raw["ts"],
				"message": raw["msg"],
				"logger":  raw["logger"],
			}
			if msg["logger"] == nil {
				msg["logger"] = raw["plugin"]
			}

			fields := make(map[string]interface{})
			for k, v := range raw {
				if k != "level" && k != "ts" && k != "msg" && k != "logger" && k != "plugin" && k != "caller" {
					fields[k] = v
				}
			}
			if len(fields) > 0 {
				msg["fields"] = fields
			}

			result = append(result, msg)
		}
	}

	// Ensure we only return the last 'lines' elements
	if len(result) > lines {
		result = result[len(result)-lines:]
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": result,
	})
}

// handleDownloadLogFile downloads a specific log file
func (s *Server) handleDownloadLogFile(r *ghttp.Request) {
	filename := r.Get("name").String()
	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		r.Response.WriteStatus(400)
		r.Response.Write("Invalid filename")
		return
	}

	dir := s.Config.Log.Dir
	if dir == "" {
		dir = "./data/logs"
	}

	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		r.Response.WriteStatus(404)
		r.Response.Write("File not found")
		return
	}

	r.Response.ServeFile(path)
}

// handleReadLogFile reads a specific log file content (tail or full)
func (s *Server) handleReadLogFile(r *ghttp.Request) {
	filename := r.Get("name").String()
	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid filename"})
		return
	}

	dir := s.Config.Log.Dir
	if dir == "" {
		dir = "./data/logs"
	}

	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		r.Response.WriteJson(g.Map{"code": 404, "message": "File not found"})
		return
	}

	// For simplicity, read the whole file if it's not too large.
	// In production, we might want to implement pagination or tail.
	content, err := os.ReadFile(path)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": string(content),
	})
}

// handleRealtimeLogs attaches a WebSocket to stream real-time logs
func (s *Server) handleRealtimeLogs(r *ghttp.Request) {
	ws, err := r.WebSocket()
	if err != nil {
		s.Logger.Error("WebSocket Upgrade failed", zap.Error(err))
		r.Exit()
	}
	defer ws.Close()

	ch := make(chan LogMessage, 100)
	GlobalLogBroadcaster.AddListener(ch)
	defer GlobalLogBroadcaster.RemoveListener(ch)

	// Send initial connection success message
	ws.WriteMessage(1, []byte(`{"type":"sys","message":"Connected to log stream"}`))

	// Read loop to keep connection alive and detect close
	done := make(chan struct{})
	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
		close(done)
	}()

	for {
		select {
		case msg := <-ch:
			data, _ := json.Marshal(msg)
			if err := ws.WriteMessage(1, data); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
