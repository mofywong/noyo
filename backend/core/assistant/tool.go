package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type ParamType string

const (
	ParamString  ParamType = "string"
	ParamNumber  ParamType = "number"
	ParamInteger ParamType = "integer"
	ParamBoolean ParamType = "boolean"
	ParamArray   ParamType = "array"
	ParamObject  ParamType = "object"
)

type Parameter struct {
	Type      ParamType
	Elem      *Parameter
	SubParams map[string]*Parameter
	Desc      string
	Enum      []string
	Required  bool
}

type ToolHandler func(ctx context.Context, arguments json.RawMessage) (any, error)

type ToolDefinition struct {
	Name    string
	Desc    string
	Params  map[string]*Parameter
	Handler ToolHandler
}

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*registeredTool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]*registeredTool)}
}

func (r *ToolRegistry) Register(def ToolDefinition) error {
	if strings.TrimSpace(def.Name) == "" {
		return fmt.Errorf("assistant tool name is required")
	}
	if def.Handler == nil {
		return fmt.Errorf("assistant tool %q handler is required", def.Name)
	}

	t := &registeredTool{
		info: &schema.ToolInfo{
			Name:        def.Name,
			Desc:        def.Desc,
			ParamsOneOf: schema.NewParamsOneOfByParams(toSchemaParams(def.Params)),
		},
		handler: def.Handler,
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("assistant tool %q already registered", def.Name)
	}
	r.tools[def.Name] = t
	return nil
}

func (r *ToolRegistry) EinoTools() []tool.BaseTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]tool.BaseTool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

func (r *ToolRegistry) ToolInfos(ctx context.Context) ([]*schema.ToolInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]*schema.ToolInfo, 0, len(r.tools))
	for _, t := range r.tools {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (r *ToolRegistry) NewToolsNode(ctx context.Context) (*compose.ToolsNode, error) {
	return compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools:               r.EinoTools(),
		ExecuteSequentially: true,
	})
}

type registeredTool struct {
	info    *schema.ToolInfo
	handler ToolHandler
}

func (t *registeredTool) Info(context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *registeredTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	result, err := t.handler(ctx, json.RawMessage(argumentsInJSON))
	if err != nil {
		return "", err
	}
	if result == nil {
		return "null", nil
	}
	if str, ok := result.(string); ok {
		return str, nil
	}
	b, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func toSchemaParams(params map[string]*Parameter) map[string]*schema.ParameterInfo {
	if len(params) == 0 {
		return nil
	}

	converted := make(map[string]*schema.ParameterInfo, len(params))
	for name, param := range params {
		converted[name] = toSchemaParam(param)
	}
	return converted
}

func toSchemaParam(param *Parameter) *schema.ParameterInfo {
	if param == nil {
		return &schema.ParameterInfo{Type: schema.String}
	}

	return &schema.ParameterInfo{
		Type:      schema.DataType(param.Type),
		ElemInfo:  toSchemaParam(param.Elem),
		SubParams: toSchemaParams(param.SubParams),
		Desc:      param.Desc,
		Enum:      param.Enum,
		Required:  param.Required,
	}
}
