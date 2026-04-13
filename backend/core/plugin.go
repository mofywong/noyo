package core

import (
	"errors"

	"noyo/core/types"
)

var ErrNotImplemented = errors.New("not implemented")
var ErrNotConnected = errors.New("client not connected")

// PluginMeta describes the plugin metadata
type PluginMeta = types.PluginMeta

// DeviceMeta encapsulates the device identity information
type DeviceMeta = types.DeviceMeta

// ProductMeta encapsulates the product information
type ProductMeta = types.ProductMeta

// TaskDefinition defines a task to be scheduled
type TaskDefinition = types.TaskDefinition

// IProtocolPlugin and IPlatformPlugin are defined in noyo/core/protocol and noyo/core/platform
// Legacy IPlugin interface has been removed - all plugins must implement the new interfaces
