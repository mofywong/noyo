package protocol

import (
	"context"
	"errors"
	"fmt"

	"noyo/core/importer"
	"noyo/core/types"
)

// BaseProtocolPlugin provides a default implementation for IProtocolPlugin
type BaseProtocolPlugin struct {
	Ctx     Context
	Meta    *types.PluginMeta
	Enabled bool
}

func (p *BaseProtocolPlugin) Init(ctx Context) error {
	p.Ctx = ctx
	return nil
}

func (p *BaseProtocolPlugin) Start() error {
	return nil
}

func (p *BaseProtocolPlugin) Stop() error {
	return nil
}

func (p *BaseProtocolPlugin) BatchAddDevice(devices []types.DeviceMeta) error {
	return nil
}

func (p *BaseProtocolPlugin) RemoveDevice(deviceCode string) error {
	return nil
}

func (p *BaseProtocolPlugin) WritePoint(device types.DeviceMeta, pointCode string, value interface{}) error {
	return errors.New("write point not implemented")
}

func (p *BaseProtocolPlugin) GetProductConfigSchema() ([]byte, error) {
	return nil, nil // Return nil to indicate no schema or empty
}

func (p *BaseProtocolPlugin) GetDeviceConfigSchema(config types.DeviceMeta) ([]byte, error) {
	return nil, nil
}

func (p *BaseProtocolPlugin) GetPointConfigSchema() ([]byte, error) {
	return nil, nil
}

// SubDeviceConfigCustomizable 默认返回 true，允许用户在产品上自定义子设备配置参数
func (p *BaseProtocolPlugin) SubDeviceConfigCustomizable() bool {
	return true
}

// ProtocolMappingRequired 默认返回 true，标准协议需要物模型映射
func (p *BaseProtocolPlugin) ProtocolMappingRequired() bool {
	return true
}

func (p *BaseProtocolPlugin) GetImportTemplateLayout(lang string) []importer.SheetLayout {
	return nil
}

func (p *BaseProtocolPlugin) ResolveImportData(ctx context.Context, rawData importer.ImportRawData) (*importer.ImportResult, error) {
	return nil, errors.New("import not implemented")
}

func (p *BaseProtocolPlugin) GetImportSampleData(products []types.ProductMeta) (*importer.ImportRawData, error) {
	return nil, nil
}

func (p *BaseProtocolPlugin) WriteProperty(device types.DeviceMeta, propName string, value interface{}) error {
	return p.WritePoint(device, propName, value)
}

func (p *BaseProtocolPlugin) CallService(device types.DeviceMeta, serviceCode string, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("CallService not implemented")
}

func (p *BaseProtocolPlugin) Discover(params map[string]interface{}) ([]DiscoveredDevice, error) {
	return nil, errors.New("discovery not implemented")
}

func (p *BaseProtocolPlugin) GetMeta() *types.PluginMeta {
	return p.Meta
}

func (p *BaseProtocolPlugin) SetMeta(meta *types.PluginMeta) {
	p.Meta = meta
}

func (p *BaseProtocolPlugin) IsEnabled() bool {
	return p.Enabled
}

func (p *BaseProtocolPlugin) SetEnabled(enabled bool) {
	p.Enabled = enabled
}

// Helper Methods for Plugins that embed BaseProtocolPlugin

func (p *BaseProtocolPlugin) ReportProperty(deviceCode string, key string, value interface{}) error {
	return p.Ctx.ReportProperty(types.DeviceMeta{DeviceCode: deviceCode}, key, value)
}

func (p *BaseProtocolPlugin) ReportEvent(deviceCode string, eventCode string, params map[string]interface{}) error {
	return p.Ctx.ReportEvent(types.DeviceMeta{DeviceCode: deviceCode}, eventCode, params)
}

func (p *BaseProtocolPlugin) ReportOnline(deviceCode string, online bool) error {
	return p.Ctx.ReportOnline(types.DeviceMeta{DeviceCode: deviceCode}, online)
}

func (p *BaseProtocolPlugin) LogInfo(msg string, fields ...interface{}) {
	p.Ctx.LogInfo(msg, fields...)
}

func (p *BaseProtocolPlugin) LogError(msg string, err error) {
	p.Ctx.LogError(msg, err)
}
