package dataquery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
)

func Execute(ctx context.Context, plan Plan, input Input, reader Reader) (Result, error) {
	if err := Validate(plan); err != nil {
		return Result{}, err
	}
	if input.Timezone == nil {
		input.Timezone, _ = location(plan.Timezone)
	}
	values := map[string][]Row{}
	uses := map[string]int{plan.Output: 1}
	for _, step := range plan.Steps {
		uses[step.Input]++
		if step.Right != "" {
			uses[step.Right]++
		}
	}
	release := func(id string) {
		uses[id]--
		if uses[id] == 0 {
			delete(values, id)
		}
	}
	result := Result{SchemaVersion: "noyo.data-result/1", Rows: []Row{}, Fields: []Field{}, Meta: Meta{Quality: "good", Completeness: "complete", GeneratedAt: time.Now().UnixMilli(), From: input.From, To: input.To, Timezone: input.Timezone.String(), Warnings: []string{}}}
	bytesRead := 0
	for _, source := range plan.Sources {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		var rows []Row
		var err error
		if source.Kind == "static" {
			rows = source.Rows
		} else {
			rows, err = reader(ctx, source, input)
		}
		if err != nil {
			return Result{}, err
		}
		if err = checkRows(rows); err != nil {
			return Result{}, err
		}
		result.Meta.ScannedRows += len(rows)
		if result.Meta.ScannedRows > MaxScanRows {
			return Result{}, ErrBudget
		}
		encoded, _ := json.Marshal(rows)
		bytesRead += len(encoded)
		if bytesRead > MaxBytes {
			return Result{}, ErrBudget
		}
		for _, row := range rows {
			if ts, ok := number(row["ts"]); ok && int64(ts) > result.Meta.SourceObservedAt {
				result.Meta.SourceObservedAt = int64(ts)
			}
		}
		if uses[source.ID] > 0 {
			values[source.ID] = rows
		}
	}
	for _, step := range plan.Steps {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		rows, err := executeStep(ctx, step, values[step.Input], values[step.Right], input)
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", step.ID, err)
		}
		if err = checkRows(rows); err != nil {
			return Result{}, err
		}
		values[step.ID] = rows
		release(step.Input)
		if step.Right != "" {
			release(step.Right)
		}
	}
	result.Rows = values[plan.Output]
	if result.Rows == nil {
		result.Rows = []Row{}
	}
	limit := plan.MaxRows
	if limit == 0 {
		limit = 1000
	}
	if len(result.Rows) > limit {
		return Result{}, ErrBudget
	}
	if len(result.Rows) == 0 {
		result.Meta.Quality = "no_data"
	}
	fields := map[string]string{}
	for _, row := range result.Rows {
		for key, v := range row {
			typ := "string"
			if v == nil {
				continue
			}
			if _, ok := number(v); ok {
				typ = "number"
			}
			if _, ok := v.(bool); ok {
				typ = "boolean"
			}
			if existing := fields[key]; existing != "" && existing != typ {
				return Result{}, errors.New("Inconsistent output field types / 输出字段类型不一致")
			}
			fields[key] = typ
		}
	}
	// Preserve null-only columns as nullable fields rather than dropping them.
	for _, row := range result.Rows {
		for key := range row {
			if fields[key] == "" {
				fields[key] = "null"
			}
		}
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		result.Fields = append(result.Fields, Field{ID: name, Type: fields[name]})
	}
	return result, nil
}

func cloneRow(row Row) Row {
	result := make(Row, len(row)+1)
	for k, v := range row {
		result[k] = v
	}
	return result
}
func keyOf(row Row, fields []string) string {
	values := make([]interface{}, len(fields))
	for i, f := range fields {
		values[i] = row[f]
	}
	data, _ := json.Marshal(values)
	return string(data)
}
func compare(a, b interface{}) int {
	if a == nil {
		if b == nil {
			return 0
		}
		return -1
	}
	if b == nil {
		return 1
	}
	if x, ok := number(a); ok {
		if y, ok := number(b); ok {
			if x < y {
				return -1
			}
			if x > y {
				return 1
			}
			return 0
		}
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return strings.Compare(string(x), string(y))
}
func executeStep(ctx context.Context, step Step, rows, right []Row, input Input) ([]Row, error) {
	result := []Row{}
	switch step.Op {
	case "filter", "derive":
		expr, _ := compileExpression(step.Expression)
		for i, row := range rows {
			if i%256 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			value, err := evaluate(expr, row, input.Parameters)
			if err != nil {
				return nil, err
			}
			if step.Op == "filter" {
				if value == nil {
					continue
				}
				b, ok := value.(bool)
				if !ok {
					return nil, errors.New("Filter must return boolean / 筛选表达式必须返回布尔值")
				}
				if b {
					result = append(result, row)
				}
			} else {
				out := cloneRow(row)
				out[step.Field] = value
				result = append(result, out)
			}
		}
	case "bucket":
		for _, row := range rows {
			value, ok := number(row[step.Field])
			if !ok {
				return nil, errors.New("Time field must be a timestamp / 时间字段必须是毫秒时间戳")
			}
			stamp := int64(value)
			if step.Calendar != "" {
				t := time.UnixMilli(stamp).In(input.Timezone)
				switch step.Calendar {
				case "hour":
					stamp = t.Add(-time.Duration(t.Minute())*time.Minute - time.Duration(t.Second())*time.Second - time.Duration(t.Nanosecond())).UnixMilli()
				case "day":
					stamp = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, input.Timezone).UnixMilli()
				case "month":
					stamp = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, input.Timezone).UnixMilli()
				}
			} else {
				// Anchor fixed buckets to the epoch so rolling refreshes keep their boundaries.
				stamp = stamp / step.IntervalMs * step.IntervalMs
			}
			out := cloneRow(row)
			out[step.Field] = stamp
			result = append(result, out)
		}
	case "aggregate":
		return aggregate(ctx, step, rows)
	case "sort":
		result = append(result, rows...)
		sort.SliceStable(result, func(i, j int) bool {
			for _, order := range step.Orders {
				c := compare(result[i][order.Field], result[j][order.Field])
				if c != 0 {
					if order.Desc {
						return c > 0
					}
					return c < 0
				}
			}
			return false
		})
	case "limit":
		if len(rows) > step.Limit {
			rows = rows[:step.Limit]
		}
		result = append(result, rows...)
	case "project":
		for _, row := range rows {
			out := Row{}
			for _, f := range step.Fields {
				out[f] = row[f]
			}
			result = append(result, out)
		}
	case "union":
		if len(rows)+len(right) > MaxRows {
			return nil, ErrBudget
		}
		result = append(append(result, rows...), right...)
	case "join":
		lookup := map[string]Row{}
		for _, row := range right {
			if row[step.RightKey] == nil {
				continue
			}
			key := keyOf(row, []string{step.RightKey})
			if _, exists := lookup[key]; exists {
				return nil, errors.New("Right join key must be unique / 关联右侧键必须唯一")
			}
			lookup[key] = row
		}
		for _, row := range rows {
			var match Row
			if row[step.LeftKey] != nil {
				match = lookup[keyOf(row, []string{step.LeftKey})]
			}
			if match == nil && step.JoinType == "inner" {
				continue
			}
			out := cloneRow(row)
			for k, v := range match {
				key := step.Prefix + "_" + k
				if _, exists := out[key]; exists {
					return nil, errors.New("Join field collision / 关联字段重名")
				}
				out[key] = v
			}
			result = append(result, out)
		}
	}
	return result, nil
}

type aggregateState struct {
	sum         float64
	count       int
	min, max    float64
	first, last interface{}
	distinct    map[string]bool
	values      []float64
}
type aggregateGroup struct {
	keys     Row
	measures []aggregateState
}

func aggregate(ctx context.Context, step Step, rows []Row) ([]Row, error) {
	groups := map[string]*aggregateGroup{}
	order := []string{}
	newGroup := func(row Row) *aggregateGroup {
		g := &aggregateGroup{keys: Row{}, measures: make([]aggregateState, len(step.Measures))}
		for _, f := range step.GroupBy {
			g.keys[f] = row[f]
		}
		return g
	}
	if len(rows) == 0 && len(step.GroupBy) == 0 {
		groups["[]"] = newGroup(Row{})
		order = append(order, "[]")
	}
	for i, row := range rows {
		if i%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		key := keyOf(row, step.GroupBy)
		g := groups[key]
		if g == nil {
			if len(groups) >= MaxRows {
				return nil, ErrBudget
			}
			g = newGroup(row)
			groups[key] = g
			order = append(order, key)
		}
		for j, m := range step.Measures {
			state := &g.measures[j]
			v := row[m.Field]
			if m.Function == "count" {
				if m.Field == "" || v != nil {
					state.count++
				}
				continue
			}
			if v == nil {
				continue
			}
			if m.Function == "count_distinct" {
				if state.distinct == nil {
					state.distinct = map[string]bool{}
				}
				data, _ := json.Marshal(v)
				state.distinct[string(data)] = true
				continue
			}
			if m.Function == "first" || m.Function == "last" {
				if state.count == 0 {
					state.first = v
				}
				state.last = v
				state.count++
				continue
			}
			n, ok := number(v)
			if !ok {
				return nil, errors.New("Numeric measure received a non-numeric value / 数值统计字段包含非数值")
			}
			if state.count == 0 {
				state.min = n
				state.max = n
			}
			state.count++
			state.sum += n
			if n < state.min {
				state.min = n
			}
			if n > state.max {
				state.max = n
			}
			if m.Function == "median" {
				state.values = append(state.values, n)
			}
		}
	}
	result := make([]Row, 0, len(groups))
	for _, key := range order {
		g := groups[key]
		out := cloneRow(g.keys)
		for j, m := range step.Measures {
			s := &g.measures[j]
			var v interface{}
			switch m.Function {
			case "count":
				v = s.count
			case "count_distinct":
				v = len(s.distinct)
			default:
				if s.count > 0 {
					switch m.Function {
					case "avg":
						v = s.sum / float64(s.count)
					case "sum":
						v = s.sum
					case "min":
						v = s.min
					case "max":
						v = s.max
					case "first":
						v = s.first
					case "last":
						v = s.last
					case "median":
						sort.Float64s(s.values)
						mid := len(s.values) / 2
						if len(s.values)%2 == 0 {
							v = (s.values[mid-1] + s.values[mid]) / 2
						} else {
							v = s.values[mid]
						}
					}
				}
			}
			out[m.ID] = v
		}
		result = append(result, out)
	}
	return result, nil
}
