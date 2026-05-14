package cascade

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gogf/gf/v2/net/ghttp"
)

func (p *CascadePlugin) handleGatewaySystemConfig(r *ghttp.Request) {
	gwSn := p.getRouteParam(r, "gwSn")
	if r.Method == "GET" {
		p.writeRemoteSystemCommandResponse(r, gwSn, remoteSystemMethodConfigGet, nil, 0)
		return
	}
	if r.Method != "POST" {
		r.Response.WriteStatus(405)
		return
	}
	var config map[string]interface{}
	if err := json.Unmarshal(r.GetBody(), &config); err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "Invalid JSON"})
		return
	}
	p.writeRemoteSystemCommandResponse(r, gwSn, remoteSystemMethodConfigSet, map[string]interface{}{"config": config}, 0)
}

func (p *CascadePlugin) handleGatewayLicenseStatus(r *ghttp.Request) {
	if r.Method != "GET" {
		r.Response.WriteStatus(405)
		return
	}
	p.writeRemoteSystemCommandResponse(r, p.getRouteParam(r, "gwSn"), remoteSystemMethodLicenseStatus, nil, 200)
}

func (p *CascadePlugin) handleGatewayLicenseUpload(r *ghttp.Request) {
	if r.Method != "POST" {
		r.Response.WriteStatus(405)
		return
	}
	upload := r.GetUploadFile("file")
	if upload == nil {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "No file uploaded"})
		return
	}

	filename, err := upload.Save(os.TempDir(), true)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	path := filepath.Join(os.TempDir(), filename)
	if filepath.IsAbs(filename) {
		path = filename
	}
	defer os.Remove(path)

	content, err := os.ReadFile(path)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	p.writeRemoteSystemCommandResponse(r, p.getRouteParam(r, "gwSn"), remoteSystemMethodLicenseUpload, map[string]interface{}{
		"content_base64": base64.StdEncoding.EncodeToString(content),
	}, 200)
}

func (p *CascadePlugin) handleGatewayLogFiles(r *ghttp.Request) {
	if r.Method != "GET" {
		r.Response.WriteStatus(405)
		return
	}
	p.writeRemoteSystemCommandResponse(r, p.getRouteParam(r, "gwSn"), remoteSystemMethodLogFiles, nil, 0)
}

func (p *CascadePlugin) handleGatewayLogTail(r *ghttp.Request) {
	if r.Method != "GET" {
		r.Response.WriteStatus(405)
		return
	}
	p.writeRemoteSystemCommandResponse(r, p.getRouteParam(r, "gwSn"), remoteSystemMethodLogTail, map[string]interface{}{
		"lines": r.Get("lines", 100).Int(),
	}, 0)
}

func (p *CascadePlugin) handleGatewayLogFile(r *ghttp.Request) {
	if r.Method != "GET" {
		r.Response.WriteStatus(405)
		return
	}
	p.writeRemoteSystemCommandResponse(r, p.getRouteParam(r, "gwSn"), remoteSystemMethodLogFile, map[string]interface{}{
		"name": r.Get("name").String(),
	}, 0)
}

func (p *CascadePlugin) writeRemoteSystemCommandResponse(r *ghttp.Request, gwSn, method string, params map[string]interface{}, successCode int) {
	engine, ok := p.remotePluginEngine(r, gwSn)
	if !ok {
		return
	}
	data, err := engine.SendRemotePluginCommand(gwSn, method, "", params)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	if successCode == 200 {
		r.Response.WriteJson(map[string]interface{}{"code": 200, "data": data, "message": "success"})
		return
	}
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": data, "message": "success"})
}
