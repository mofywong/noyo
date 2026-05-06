package platform

import (
	"errors"
	"noyo/core/types"
)

// BasePlatformPlugin provides a default implementation for IPlatformPlugin
type BasePlatformPlugin struct {
	Ctx     Context
	Meta    *types.PluginMeta
	Enabled bool
}

func (p *BasePlatformPlugin) Init(ctx Context) error {
	p.Ctx = ctx
	return nil
}

func (p *BasePlatformPlugin) Start() error {
	return nil
}

func (p *BasePlatformPlugin) Stop() error {
	return nil
}

func (p *BasePlatformPlugin) PushData(data *DataModel) error {
	return nil
}

func (p *BasePlatformPlugin) GetMeta() *types.PluginMeta {
	return p.Meta
}

func (p *BasePlatformPlugin) SetMeta(meta *types.PluginMeta) {
	p.Meta = meta
}

func (p *BasePlatformPlugin) IsEnabled() bool {
	return p.Enabled
}

func (p *BasePlatformPlugin) SetEnabled(enabled bool) {
	p.Enabled = enabled
}

// Helper methods referencing Context

func (p *BasePlatformPlugin) IssueCommand(deviceCode string, cmdCode string, params map[string]interface{}) (interface{}, error) {
	if p.Ctx != nil {
		return p.Ctx.IssueCommand(deviceCode, cmdCode, params)
	}
	return nil, errors.New("context not initialized")
}

func (p *BasePlatformPlugin) LogInfo(msg string, fields ...interface{}) {
	if p.Ctx != nil {
		p.Ctx.LogInfo(msg, fields...)
	}
}

func (p *BasePlatformPlugin) LogError(msg string, err error) {
	if p.Ctx != nil {
		p.Ctx.LogError(msg, err)
	}
}
