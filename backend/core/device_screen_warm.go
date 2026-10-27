package core

import (
	"time"

	"go.uber.org/zap"
	"noyo/core/store"
	"noyo/core/tsdb"
)

// Live screen samples are held in memory by the device runtime: they exist only
// after a device reports. A restart therefore blanked every point, aggregate and
// status binding on every published screen until each device happened to report
// again — minutes for a fast device, forever for one that stopped sending.
//
// The warm-up seeds only the last known value of devices an actual screen binds,
// and keeps the record's own timestamp, so a value from three days ago is
// immediately reported as stale instead of masquerading as fresh.
const screenSampleWarmWindow = 30 * 24 * time.Hour

// screenSampleWarmDeviceLimit bounds the work a restart may do in the background.
const screenSampleWarmDeviceLimit = 500

func (s *Server) warmScreenSamples() {
	if s.DeviceManager == nil || s.TSDB == nil || store.DB == nil {
		return
	}
	codes := make([]string, 0, 64)
	if err := store.DB.Model(&store.IOCScreenResourceRef{}).
		Where("resource_type = ?", "device").
		Distinct().
		Limit(screenSampleWarmDeviceLimit).
		Pluck("resource_id", &codes).Error; err != nil {
		s.Logger.Warn("Screen sample warm-up skipped", zap.Error(err))
		return
	}
	if len(codes) == 0 {
		return
	}
	now := time.Now().UnixMilli()
	start := now - screenSampleWarmWindow.Milliseconds()
	seeded := 0
	for _, code := range codes {
		if code == "" {
			continue
		}
		response, err := s.TSDB.Query(tsdb.QueryRequest{
			DeviceCode: code,
			StartTime:  start,
			EndTime:    now,
			Type:       tsdb.TypeTelemetry,
			Page:       1,
			PageSize:   1,
		})
		if err != nil || response == nil || len(response.List) == 0 {
			continue
		}
		row, ok := response.List[0].(map[string]interface{})
		if !ok {
			continue
		}
		receivedAt, ok := screenNumber(row["ts"])
		if !ok || receivedAt <= 0 {
			continue
		}
		values := make(map[string]interface{}, len(row))
		for key, value := range row {
			if key == "ts" {
				continue
			}
			values[key] = value
		}
		if s.DeviceManager.SeedScreenSamples(code, values, int64(receivedAt)) {
			seeded++
		}
	}
	if seeded > 0 {
		s.Logger.Info("Screen samples warmed from history", zap.Int("devices", seeded), zap.Int("candidates", len(codes)))
	}
}
