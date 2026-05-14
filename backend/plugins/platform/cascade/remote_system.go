package cascade

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"noyo/core"
	"noyo/core/config"
	"noyo/core/store"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type remoteSystemConfigSetRequest struct {
	Config config.GlobalConfig `json:"config"`
}

type remoteLicenseUploadRequest struct {
	ContentBase64 string `json:"content_base64"`
	Content       string `json:"content"`
}

type remoteLogTailRequest struct {
	Lines int `json:"lines"`
}

type remoteLogFileRequest struct {
	Name string `json:"name"`
}

func getGatewaySystemConfig(coreServer *core.Server) (interface{}, error) {
	if coreServer == nil {
		return nil, fmt.Errorf("core server not available")
	}
	return coreServer.Config, nil
}

func setGatewaySystemConfig(coreServer *core.Server, params interface{}) (interface{}, error) {
	if coreServer == nil {
		return nil, fmt.Errorf("core server not available")
	}
	var req remoteSystemConfigSetRequest
	if err := scanRemotePluginParams(params, &req); err != nil {
		return nil, err
	}
	coreServer.Config.Server = req.Config.Server
	coreServer.Config.TSDB = req.Config.TSDB
	coreServer.Config.Log = req.Config.Log
	if err := store.SaveGlobalConfig(coreServer.Config); err != nil {
		return nil, err
	}
	return coreServer.Config, nil
}

func getGatewayLicenseStatus(coreServer *core.Server) (interface{}, error) {
	status := "unauthorized"
	message := "license_auth plugin is not available"
	if coreServer != nil && coreServer.Manager != nil {
		if plugin := coreServer.Manager.GetPlugin("license_auth"); plugin != nil {
			if reporter, ok := plugin.(interface{ Status() string }); ok {
				status = reporter.Status()
			} else if plugin.IsEnabled() {
				status = "authorized"
			}
			message = status
		}
	}

	resp := map[string]interface{}{
		"status":     status,
		"message":    message,
		"machine_id": readGatewayMachineID(),
	}
	return resp, nil
}

func uploadGatewayLicense(coreServer *core.Server, params interface{}) (interface{}, error) {
	var req remoteLicenseUploadRequest
	if err := scanRemotePluginParams(params, &req); err != nil {
		return nil, err
	}

	var content []byte
	if req.ContentBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.ContentBase64)
		if err != nil {
			return nil, fmt.Errorf("decode license: %w", err)
		}
		content = decoded
	} else {
		content = []byte(req.Content)
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("license content is empty")
	}

	licensePath := filepath.Join("data", "license.lic")
	if err := os.MkdirAll(filepath.Dir(licensePath), os.ModePerm); err != nil {
		return nil, err
	}
	if err := os.WriteFile(licensePath, content, 0644); err != nil {
		return nil, err
	}

	if coreServer != nil && coreServer.Manager != nil && coreServer.Manager.GetPlugin("license_auth") != nil {
		if err := coreServer.Manager.ReloadPlugin("license_auth"); err != nil {
			return nil, err
		}
	}
	return getGatewayLicenseStatus(coreServer)
}

func listGatewayLogFiles(coreServer *core.Server) (interface{}, error) {
	dir := gatewayLogDir(coreServer)
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []interface{}{}, nil
		}
		return nil, err
	}

	result := make([]map[string]interface{}, 0)
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

	sort.Slice(result, func(i, j int) bool {
		return result[i]["time"].(string) > result[j]["time"].(string)
	})
	return result, nil
}

func tailGatewayLog(coreServer *core.Server, params interface{}) (interface{}, error) {
	req := remoteLogTailRequest{Lines: 100}
	if params != nil {
		_ = scanRemotePluginParams(params, &req)
	}
	if req.Lines <= 0 {
		req.Lines = 100
	}

	path := filepath.Join(gatewayLogDir(coreServer), "noyo.log")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []interface{}{}, nil
		}
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	readSize := int64(131072)
	if stat.Size() < readSize {
		readSize = stat.Size()
	}
	offset := stat.Size() - readSize
	buf := make([]byte, readSize)
	_, err = file.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}

	result := make([]map[string]interface{}, 0)
	for _, line := range strings.Split(string(buf), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
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

	if len(result) > req.Lines {
		result = result[len(result)-req.Lines:]
	}
	return result, nil
}

func readGatewayLogFile(coreServer *core.Server, params interface{}) (interface{}, error) {
	var req remoteLogFileRequest
	if err := scanRemotePluginParams(params, &req); err != nil {
		return nil, err
	}
	if req.Name == "" || strings.Contains(req.Name, "/") || strings.Contains(req.Name, "\\") {
		return nil, fmt.Errorf("invalid filename")
	}
	content, err := os.ReadFile(filepath.Join(gatewayLogDir(coreServer), req.Name))
	if err != nil {
		return nil, err
	}
	return string(content), nil
}

func gatewayLogDir(coreServer *core.Server) string {
	if coreServer != nil && coreServer.Config.Log.Dir != "" {
		return coreServer.Config.Log.Dir
	}
	return "./data/logs"
}

func readGatewayMachineID() string {
	content, err := os.ReadFile(filepath.Join("data", ".noyo_machine_id"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}
