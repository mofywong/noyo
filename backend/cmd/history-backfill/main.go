// history-backfill previews or applies explicitly reviewed legacy ownership.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"go.uber.org/zap"
	"noyo/core/tsdb"
)

func run() error {
	dir := flag.String("dir", "", "History directory / 历史目录")
	mappingPath := flag.String("mapping", "", "Trusted mapping JSON file / 可信归属映射文件")
	apply := flag.Bool("apply", false, "Apply after preview and backup / 预览并备份后执行回填")
	flag.Parse()
	if *dir == "" || *mappingPath == "" {
		return fmt.Errorf("--dir and --mapping are required / 必须指定历史目录和映射文件")
	}
	file, err := os.Open(*mappingPath)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var mappings []tsdb.LegacyOwnershipMapping
	if err := decoder.Decode(&mappings); err != nil {
		return err
	}
	if len(mappings) == 0 || len(mappings) > 1000 {
		return fmt.Errorf("mapping must contain 1..1000 entries")
	}
	if decoder.Decode(new(interface{})) != io.EOF {
		return fmt.Errorf("mapping contains trailing data")
	}
	for i, mapping := range mappings {
		for j := 0; j < i; j++ {
			previous := mappings[j]
			if mapping.DeviceCode == previous.DeviceCode && mapping.From <= previous.To && mapping.To >= previous.From {
				return fmt.Errorf("overlapping mappings for %s / 设备映射时间范围重叠", mapping.DeviceCode)
			}
		}
	}
	storage := tsdb.NewSQLiteStore(*dir, zap.NewNop())
	defer storage.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// Validate every mapping and shard before making any change.
	for _, mapping := range mappings {
		count, err := storage.PreviewLegacyOwnership(ctx, mapping)
		if err != nil {
			return err
		}
		fmt.Printf("Preview / 预览: device=%s rows=%d owner=%d/%d/%d\n", mapping.DeviceCode, count, mapping.Owner.TenantID, mapping.Owner.ProjectID, mapping.Owner.DeviceID)
	}
	if !*apply {
		fmt.Println("Dry run only / 仅预览；未修改数据")
		return nil
	}
	for _, mapping := range mappings {
		count, err := storage.BackfillLegacyOwnership(ctx, mapping)
		if err != nil {
			return err
		}
		fmt.Printf("Applied / 已回填: device=%s rows=%d\n", mapping.DeviceCode, count)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
