// Package dataquery executes bounded, declarative queries independently of IOC
// components. Providers must authorize their inputs before returning any rows.
package dataquery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

const (
	SchemaVersion       = "noyo.dataset/1"
	MaxSources          = 8
	MaxSteps            = 24
	MaxDevices          = 200
	MaxScanRows         = 100000
	MaxRows             = 20000
	MaxResultRows       = 5000
	MaxBytes            = 16 << 20
	MaxRangeMs    int64 = 366 * 86400000
)

var ErrBudget = errors.New("Query budget exceeded / 查询超出预算，请缩小范围或增加时间粒度")
var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.:-]{0,127}$`)
var deviceIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

type Row = map[string]interface{}
type Parameter struct {
	Type     string        `json:"type"`
	Default  interface{}   `json:"default,omitempty"`
	Required bool          `json:"required,omitempty"`
	Options  []interface{} `json:"options,omitempty"`
}
type Selector struct {
	DeviceCodes      []string `json:"deviceCodes,omitempty"`
	ProductCode      string   `json:"productCode,omitempty"`
	ProductParameter string   `json:"productParameter,omitempty"`
	TagIDs           []uint   `json:"tagIds,omitempty"`
}
type Source struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Selector Selector        `json:"selector,omitempty"`
	Fields   []string        `json:"fields,omitempty"`
	Rows     []Row           `json:"rows,omitempty"`
	Config   json.RawMessage `json:"config,omitempty"`
}
type Measure struct {
	ID       string `json:"id"`
	Function string `json:"function"`
	Field    string `json:"field,omitempty"`
}
type Order struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc,omitempty"`
}
type Step struct {
	ID         string    `json:"id"`
	Op         string    `json:"op"`
	Input      string    `json:"input"`
	Right      string    `json:"right,omitempty"`
	Expression string    `json:"expression,omitempty"`
	Field      string    `json:"field,omitempty"`
	Fields     []string  `json:"fields,omitempty"`
	GroupBy    []string  `json:"groupBy,omitempty"`
	Measures   []Measure `json:"measures,omitempty"`
	IntervalMs int64     `json:"intervalMs,omitempty"`
	Calendar   string    `json:"calendar,omitempty"`
	Orders     []Order   `json:"orders,omitempty"`
	Limit      int       `json:"limit,omitempty"`
	LeftKey    string    `json:"leftKey,omitempty"`
	RightKey   string    `json:"rightKey,omitempty"`
	JoinType   string    `json:"joinType,omitempty"`
	Prefix     string    `json:"prefix,omitempty"`
}
type Plan struct {
	SchemaVersion string               `json:"schemaVersion"`
	Sources       []Source             `json:"sources"`
	Steps         []Step               `json:"steps"`
	Output        string               `json:"output"`
	Parameters    map[string]Parameter `json:"parameters,omitempty"`
	WindowMs      int64                `json:"windowMs,omitempty"`
	Timezone      string               `json:"timezone,omitempty"`
	MaxRows       int                  `json:"maxRows,omitempty"`
}
type Field struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
type Meta struct {
	Quality          string   `json:"quality"`
	Completeness     string   `json:"completeness"`
	GeneratedAt      int64    `json:"generatedAt"`
	SourceObservedAt int64    `json:"sourceObservedAt,omitempty"`
	From             int64    `json:"from"`
	To               int64    `json:"to"`
	Timezone         string   `json:"timezone"`
	ScannedRows      int      `json:"scannedRows"`
	CacheHit         bool     `json:"cacheHit"`
	Warnings         []string `json:"warnings"`
}
type Result struct {
	SchemaVersion string  `json:"schemaVersion"`
	Fields        []Field `json:"fields"`
	Rows          []Row   `json:"rows"`
	Meta          Meta    `json:"meta"`
}
type Input struct {
	From       int64
	To         int64
	Parameters map[string]interface{}
	Timezone   *time.Location
}
type Reader func(context.Context, Source, Input) ([]Row, error)

func validName(name string) bool {
	return identifier.MatchString(name) && !strings.Contains(name, "__proto__") && name != "constructor" && name != "prototype"
}
func Validate(p Plan) error {
	if p.SchemaVersion != SchemaVersion || len(p.Sources) == 0 || len(p.Sources) > MaxSources || len(p.Steps) > MaxSteps || p.WindowMs < 0 || p.WindowMs > MaxRangeMs || p.MaxRows < 0 || p.MaxRows > MaxResultRows || len(p.Parameters) > 16 {
		return errors.New("Invalid dataset or query limits / 数据集或查询限制无效")
	}
	if _, err := location(p.Timezone); err != nil {
		return err
	}
	seen := map[string]bool{}
	for name, param := range p.Parameters {
		if !validName(name) || len(param.Options) > 200 || !oneOf(param.Type, "string", "number", "boolean") {
			return errors.New("Invalid parameter / 参数无效")
		}
		if param.Default != nil && !parameterType(param.Default, param.Type) {
			return errors.New("Invalid parameter default / 参数默认值类型无效")
		}
		for _, option := range param.Options {
			if !parameterType(option, param.Type) {
				return errors.New("Invalid parameter option / 参数选项类型无效")
			}
		}
	}
	for _, src := range p.Sources {
		if !validName(src.ID) || seen[src.ID] || !validName(src.Kind) || len(src.Fields) > 64 || len(src.Selector.DeviceCodes) > MaxDevices || len(src.Selector.TagIDs) > 32 || len(src.Rows) > MaxRows || len(src.Config) > 65536 {
			return errors.New("Invalid data source / 数据源无效")
		}
		for _, f := range src.Fields {
			if !validName(f) {
				return errors.New("Invalid field / 字段无效")
			}
		}
		for _, code := range src.Selector.DeviceCodes {
			if !deviceIdentifier.MatchString(code) {
				return errors.New("Invalid device code / 设备编码无效")
			}
		}
		if src.Selector.ProductParameter != "" {
			if param, ok := p.Parameters[src.Selector.ProductParameter]; !ok || param.Type != "string" {
				return errors.New("Product parameter must be declared as a string / 产品参数须声明为字符串")
			}
		}
		if src.Kind != "static" && len(src.Rows) > 0 {
			return errors.New("Only static sources accept inline rows / 仅静态数据源可包含内联数据")
		}
		if src.Kind == "static" {
			if err := checkRows(src.Rows); err != nil {
				return err
			}
		}
		seen[src.ID] = true
	}
	for _, step := range p.Steps {
		if !validName(step.ID) || seen[step.ID] || !seen[step.Input] {
			return errors.New("Invalid query dependency / 查询依赖无效")
		}
		if len(step.Fields) > 64 || len(step.GroupBy) > 8 || len(step.Measures) > 32 || len(step.Orders) > 8 {
			return ErrBudget
		}
		for _, f := range append(append([]string{}, step.Fields...), step.GroupBy...) {
			if !validName(f) {
				return errors.New("Invalid field / 字段无效")
			}
		}
		switch step.Op {
		case "filter", "derive":
			if _, err := compileExpression(step.Expression); err != nil {
				return err
			}
			if step.Op == "derive" && !validName(step.Field) {
				return errors.New("Calculated field is required / 需要计算字段名")
			}
		case "bucket":
			if !validName(step.Field) || (step.Calendar != "" && !oneOf(step.Calendar, "hour", "day", "month")) || (step.Calendar == "" && (step.IntervalMs < 1000 || step.IntervalMs > MaxRangeMs)) {
				return errors.New("Invalid time bucket / 时间分桶无效")
			}
		case "aggregate":
			if len(step.Measures) == 0 {
				return errors.New("Measures are required / 需要统计字段")
			}
			names := map[string]bool{}
			for _, f := range step.GroupBy {
				names[f] = true
			}
			for _, m := range step.Measures {
				if !validName(m.ID) || names[m.ID] || !oneOf(m.Function, "avg", "sum", "min", "max", "count", "count_distinct", "first", "last", "median") || (m.Field == "" && m.Function != "count") || (m.Field != "" && !validName(m.Field)) {
					return errors.New("Invalid measure / 统计字段无效")
				}
				names[m.ID] = true
			}
		case "sort":
			for _, o := range step.Orders {
				if !validName(o.Field) {
					return errors.New("Invalid sort field / 排序字段无效")
				}
			}
		case "limit":
			if step.Limit < 1 || step.Limit > MaxResultRows {
				return ErrBudget
			}
		case "project":
			if len(step.Fields) == 0 {
				return errors.New("Output fields are required / 需要输出字段")
			}
		case "join", "union":
			if !seen[step.Right] {
				return errors.New("Unknown right input / 右侧输入不存在")
			}
			if step.Op == "join" && (!validName(step.LeftKey) || !validName(step.RightKey) || !validName(step.Prefix) || !oneOf(step.JoinType, "left", "inner")) {
				return errors.New("Invalid join keys or prefix / 关联键或前缀无效")
			}
		default:
			return errors.New("Unsupported query operation / 不支持的查询操作")
		}
		seen[step.ID] = true
	}
	if !seen[p.Output] {
		return errors.New("Unknown dataset output / 数据集输出不存在")
	}
	return nil
}
func location(name string) (*time.Location, error) {
	if name == "" {
		name = "UTC"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, errors.New("Invalid timezone / 时区无效")
	}
	return loc, nil
}
func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
func parameterType(v interface{}, t string) bool {
	switch t {
	case "string":
		s, ok := v.(string)
		return ok && len(s) <= 256
	case "number":
		_, ok := number(v)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	}
	return false
}
func Resolve(p Plan, values map[string]interface{}, from, to, now int64) (Input, error) {
	if err := Validate(p); err != nil {
		return Input{}, err
	}
	resolved := map[string]interface{}{}
	for name := range values {
		if _, ok := p.Parameters[name]; !ok {
			return Input{}, fmt.Errorf("Unknown parameter / 未知参数: %s", name)
		}
	}
	for name, param := range p.Parameters {
		v, ok := values[name]
		if !ok {
			v = param.Default
		}
		if v == nil {
			if param.Required {
				return Input{}, fmt.Errorf("Required parameter / 缺少参数: %s", name)
			}
			continue
		}
		if !parameterType(v, param.Type) {
			return Input{}, fmt.Errorf("Parameter type mismatch / 参数类型不匹配: %s", name)
		}
		if len(param.Options) > 0 {
			matched := false
			for _, option := range param.Options {
				a, _ := json.Marshal(option)
				b, _ := json.Marshal(v)
				matched = matched || string(a) == string(b)
			}
			if !matched {
				return Input{}, fmt.Errorf("Parameter outside allowed values / 参数超出允许范围: %s", name)
			}
		}
		resolved[name] = v
	}
	if to == 0 {
		to = now
	}
	if from == 0 {
		window := p.WindowMs
		if window == 0 {
			window = 3600000
		}
		from = to - window
	}
	if from < 0 || to <= from || to-from > MaxRangeMs || to > now+60000 {
		return Input{}, errors.New("Invalid query time range / 查询时间范围无效")
	}
	loc, _ := location(p.Timezone)
	return Input{From: from, To: to, Parameters: resolved, Timezone: loc}, nil
}
func number(v interface{}) (float64, bool) {
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case float32:
		n = float64(x)
	case int:
		n = float64(x)
	case int64:
		n = float64(x)
	case uint:
		n = float64(x)
	case uint64:
		if x > 1<<53 {
			return 0, false
		}
		n = float64(x)
	case json.Number:
		var err error
		n, err = x.Float64()
		if err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	return n, !math.IsNaN(n) && !math.IsInf(n, 0)
}
func checkRows(rows []Row) error {
	if len(rows) > MaxScanRows {
		return ErrBudget
	}
	for _, row := range rows {
		if len(row) > 64 {
			return ErrBudget
		}
		for key, value := range row {
			if !validName(key) {
				return errors.New("Invalid result field / 结果字段无效")
			}
			switch x := value.(type) {
			case nil, bool:
			case string:
				if len(x) > 4096 {
					return ErrBudget
				}
			default:
				if _, ok := number(value); !ok {
					return errors.New("Rows must contain scalar values / 数据行必须使用标量值")
				}
			}
		}
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	if len(data) > MaxBytes {
		return ErrBudget
	}
	return nil
}
