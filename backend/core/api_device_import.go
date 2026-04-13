package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/importer"
	"noyo/core/protocol"
	"noyo/core/store"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
)

// handleDownloadTemplate handles template download request
func (s *Server) handleDownloadTemplate(r *ghttp.Request) {
	protocolName := r.GetQuery("protocol").String()
	productCodesStr := r.GetQuery("product_codes").String()

	// 1. Get Plugin
	if protocolName == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Protocol is required"})
		return
	}
	plugin := s.Manager.GetPlugin(protocolName)
	if plugin == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": fmt.Sprintf("Plugin %s not found", protocolName)})
		return
	}
	protoPlugin, ok := plugin.(protocol.IProtocolPlugin)
	if !ok {
		r.Response.WriteJson(g.Map{"code": 400, "message": fmt.Sprintf("Plugin %s does not support device import", protocolName)})
		return
	}

	// 2. Get Layout
	lang := r.GetQuery("lang").String()
	if lang == "" {
		lang = "zh"
	}
	layout := protoPlugin.GetImportTemplateLayout(lang)

	// 3. Prepare Injection Data (Products)
	var products []store.Product
	if productCodesStr != "" {
		codes := strings.Split(productCodesStr, ",")
		if len(codes) > 0 {
			store.DB.Where("code IN ?", codes).Find(&products)
		}
	} else {
		store.DB.Where("protocol_name = ?", protocolName).Find(&products)
	}

	// Convert store.Product to names/codes for dropdowns
	productOptions := make([]string, len(products))
	productMetas := make([]ProductMeta, len(products))
	for i, p := range products {
		productOptions[i] = fmt.Sprintf("%s (%s)", p.Name, p.Code)

		var config map[string]interface{}
		json.Unmarshal([]byte(p.Config), &config)
		productMetas[i] = ProductMeta{
			Name:   p.Name,
			Code:   p.Code,
			Config: config,
		}
	}

	injection := importer.InjectionData{
		"target_products":   productOptions,
		"selected_products": productOptions, // Alias
	}

	// 4. Generate Template Structure
	f, err := importer.GenerateTemplate(layout, injection)
	if err != nil {
		s.Logger.Error("Failed to generate template", zap.Error(err))
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to generate template"})
		return
	}

	// 5. Pre-fill Data (Sample Data)
	// We call GetImportSampleData from plugin
	sampleData, err := protoPlugin.GetImportSampleData(productMetas)
	if err == nil && sampleData != nil {
		// Fill data
		if err := importer.FillData(f, layout, *sampleData); err != nil {
			s.Logger.Warn("Failed to fill sample data", zap.Error(err))
		}
	}

	// 6. Write Response
	r.Response.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	r.Response.Header().Set("Content-Disposition", "attachment; filename=device_import_template.xlsx")
	if err := f.Write(r.Response.Writer); err != nil {
		s.Logger.Error("Failed to write excel", zap.Error(err))
	}
}

// handleImportDevices handles device import
func (s *Server) handleImportDevices(r *ghttp.Request) {
	protocolName := r.GetQuery("protocol").String()

	// 1. Get Plugin
	if protocolName == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Protocol is required"})
		return
	}
	plugin := s.Manager.GetPlugin(protocolName)
	if plugin == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": fmt.Sprintf("Plugin %s not found", protocolName)})
		return
	}
	protoPlugin, ok := plugin.(protocol.IProtocolPlugin)
	if !ok {
		r.Response.WriteJson(g.Map{"code": 400, "message": fmt.Sprintf("Plugin %s does not support device import", protocolName)})
		return
	}

	// 2. Parse File
	file, _, err := r.Request.FormFile("file")
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "File is required"})
		return
	}
	defer file.Close()

	raw, err := importer.Parse(file)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": fmt.Sprintf("Failed to parse excel: %v", err)})
		return
	}
	s.Logger.Info("Import: File parsed", zap.Int("sheets", len(raw)))

	// 3. Normalize Data (Header -> Key)
	// We load both languages to support importing files in either language
	layoutZh := protoPlugin.GetImportTemplateLayout("zh")
	layoutEn := protoPlugin.GetImportTemplateLayout("en")

	// Merge layouts to support both headers
	combinedLayout := layoutZh
	if len(combinedLayout) == len(layoutEn) {
		for i := range combinedLayout {
			if combinedLayout[i].Name == layoutEn[i].Name {
				combinedLayout[i].Columns = append(combinedLayout[i].Columns, layoutEn[i].Columns...)
			}
		}
	}

	normalizedRaw := importer.Normalize(raw, combinedLayout)
	s.Logger.Info("Import: Data normalized")

	// 4. Resolve Data via Plugin
	res, err := protoPlugin.ResolveImportData(r.Context(), normalizedRaw)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": fmt.Sprintf("Import failed: %v", err)})
		return
	}
	s.Logger.Info("Import: Data resolved",
		zap.Int("devices_found", len(res.Devices)),
		zap.Int("errors_found", len(res.Errors)))

	// 5. Save Results
	count := 0
	errs := res.Errors
	parentsToRestart := make(map[string]bool)

	for _, devModel := range res.Devices {
		// Verify Product
		var product store.Product
		if err := store.DB.Where("code = ?", devModel.ProductCode).First(&product).Error; err != nil {
			errs = append(errs, fmt.Sprintf("Product %s not found for device %s", devModel.ProductCode, devModel.Name))
			continue
		}

		// Create Device
		device := store.Device{
			Name:        devModel.Name,
			Code:        devModel.Code,
			ProductCode: devModel.ProductCode,
			ParentCode:  devModel.ParentCode,
			Enabled:     devModel.Enabled,
		}

		// Merge Config
		config := devModel.Config
		if config == nil {
			config = make(map[string]interface{})
		}

		// If Points exist, add to config (Standard Pattern)
		if len(devModel.Points) > 0 {
			config["points"] = devModel.Points
		}

		configBytes, _ := json.Marshal(config)
		device.Config = string(configBytes)

		if err := store.SaveDevice(&device); err != nil {
			errs = append(errs, fmt.Sprintf("Failed to save device %s: %v", devModel.Name, err))
			continue
		}

		// Update Registry Cache
		s.DeviceManager.Registry.UpdateDevice(&device)

		// Handle Start/Restart Logic
		if device.ParentCode != "" {
			parentsToRestart[device.ParentCode] = true
		} else if device.Enabled {
			if err := s.DeviceManager.StartDevice(device.Code); err != nil {
				s.Logger.Error("Failed to auto-start imported device", zap.String("code", device.Code), zap.Error(err))
			}
		}

		count++
	}

	// Restart impacted parents
	for pCode := range parentsToRestart {
		s.restartParent(pCode)
	}

	s.Logger.Info("Import: Completed", zap.Int("success", count), zap.Int("errors", len(errs)))

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": g.Map{
			"success_count": count,
			"errors":        errs,
		},
	})
}
