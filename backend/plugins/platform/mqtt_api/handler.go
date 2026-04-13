package mqtt_api

import (
	"encoding/json"
	"noyo/core/store"

	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func (p *MQTTAPIPlugin) handleGatewayCommand(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	if strings.HasSuffix(topic, "_reply") {
		return
	}

	parts := strings.Split(topic, "/")
	// /sys/{GatewayCode}/api/{serviceId}
	// 0 / 1 / 2          / 3 / 4
	if len(parts) < 5 {
		return
	}

	serviceId := parts[4]

	var cmd Payload
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return
	}

	reply := CommandReply{
		ID:        cmd.ID,
		Timestamp: time.Now().UnixMilli(),
		Code:      200,
		Message:   "success",
	}

	// Helper function to extract int from interface
	getInt := func(val interface{}, defaultVal int) int {
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		default:
			return defaultVal
		}
	}

	switch serviceId {
	case "device_list_get":
		// 分页获取网关的设备信息列表（可支持不分页）
		// 分页获取网关下指定多个产品编码或多个产品名称的设备信息列表（可支持不分页）
		params, _ := cmd.Params.(map[string]interface{})
		page := getInt(params["page"], 0)
		pageSize := getInt(params["pageSize"], 0)

		var productCodes []string
		if pcs, ok := params["product_codes"].([]interface{}); ok {
			for _, pc := range pcs {
				if s, ok := pc.(string); ok {
					productCodes = append(productCodes, s)
				}
			}
		}

		var productNames []string
		if pns, ok := params["product_names"].([]interface{}); ok {
			for _, pn := range pns {
				if s, ok := pn.(string); ok {
					productNames = append(productNames, s)
				}
			}
		}

		devices, total, err := store.ListDevices(page, pageSize)
		if err != nil {
			reply.Code = 500
			reply.Message = err.Error()
		} else {
			// Filter by product codes or names if provided
			var filteredDevices []store.Device
			if len(productCodes) > 0 || len(productNames) > 0 {
				for _, d := range devices {
					match := false
					if len(productCodes) > 0 {
						for _, pc := range productCodes {
							if d.ProductCode == pc {
								match = true
								break
							}
						}
					}
					if !match && len(productNames) > 0 {
						// Need to lookup product name
						if p, _ := store.GetProduct(d.ProductCode); p != nil {
							for _, pn := range productNames {
								if p.Name == pn {
									match = true
									break
								}
							}
						}
					}
					if match {
						filteredDevices = append(filteredDevices, d)
					}
				}
				// If filtered, total should be length of filtered
				// If pagination was used, this simple filter might not work well with DB limit/offset.
				// But since it's a plugin, we'll do our best.
				devices = filteredDevices
				total = int64(len(filteredDevices))
			}

			reply.Data = map[string]interface{}{
				"total": total,
				"list":  devices,
			}
		}

	case "product_list_get":
		// 分页获取网关的产品信息列表（可支持不分页）
		params, _ := cmd.Params.(map[string]interface{})
		page := getInt(params["page"], 0)
		pageSize := getInt(params["pageSize"], 0)

		products, total, err := store.ListProducts(page, pageSize)
		if err != nil {
			reply.Code = 500
			reply.Message = err.Error()
		} else {
			reply.Data = map[string]interface{}{
				"total": total,
				"list":  products,
			}
		}

	case "product_tsl_get":
		// 获取指定多个产品的物模型定义
		params, _ := cmd.Params.(map[string]interface{})
		var productCodes []string
		if pcs, ok := params["product_codes"].([]interface{}); ok {
			for _, pc := range pcs {
				if s, ok := pc.(string); ok {
					productCodes = append(productCodes, s)
				}
			}
		}

		tslMap := make(map[string]interface{})
		for _, pc := range productCodes {
			if prod, err := store.GetProduct(pc); err == nil && prod != nil {
				var configMap map[string]interface{}
				if err := json.Unmarshal([]byte(prod.Config), &configMap); err == nil {
					if tsl, ok := configMap["tsl"]; ok {
						tslMap[pc] = tsl
					}
				}
			}
		}
		reply.Data = tslMap

	case "device_property_get":
		// 获取指定多个设备的全部属性实时值
		params, _ := cmd.Params.(map[string]interface{})
		var deviceCodes []string
		if dcs, ok := params["device_codes"].([]interface{}); ok {
			for _, dc := range dcs {
				if s, ok := dc.(string); ok {
					deviceCodes = append(deviceCodes, s)
				}
			}
		}

		dataMap := make(map[string]interface{})
		for _, dc := range deviceCodes {
			data := p.Ctx.GetDeviceData(dc)
			if data != nil {
				dataMap[dc] = data
			}
		}
		reply.Data = dataMap

	case "property_set":
		// 单设备属性控制下发
		deviceCode := cmd.DeviceCode
		if deviceCode == "" {
			reply.Code = 400
			reply.Message = "deviceCode is required"
			break
		}

		paramsMap, _ := cmd.Params.(map[string]interface{})
		_, err := p.Ctx.IssueCommand(deviceCode, "set_properties", paramsMap)
		if err != nil {
			reply.Code = 500
			reply.Message = err.Error()
		}

	case "service_invoke":
		// 单设备服务控制下发
		deviceCode := cmd.DeviceCode
		if deviceCode == "" {
			reply.Code = 400
			reply.Message = "deviceCode is required"
			break
		}

		params, _ := cmd.Params.(map[string]interface{})
		serviceCode, _ := params["service_id"].(string)
		invokeParams, _ := params["params"].(map[string]interface{})

		if serviceCode == "" {
			reply.Code = 400
			reply.Message = "service_id is required in params"
			break
		}

		res, err := p.Ctx.IssueCommand(deviceCode, serviceCode, invokeParams)
		if err != nil {
			reply.Code = 500
			reply.Message = err.Error()
		} else {
			reply.Data = res
		}

	case "property_batch_set":
		// 同产品多设备的属性控制下发接口
		productCode := cmd.ProductCode
		if productCode == "" {
			reply.Code = 400
			reply.Message = "productCode is required"
			break
		}

		params, _ := cmd.Params.(map[string]interface{})
		var deviceCodes []string
		if dcs, ok := params["device_codes"].([]interface{}); ok {
			for _, dc := range dcs {
				if s, ok := dc.(string); ok {
					deviceCodes = append(deviceCodes, s)
				}
			}
		}

		if len(deviceCodes) == 0 {
			reply.Code = 400
			reply.Message = "device_codes is required and cannot be empty"
			break
		}

		setParams, _ := params["params"].(map[string]interface{})
		if len(setParams) == 0 {
			reply.Code = 400
			reply.Message = "params is required and cannot be empty"
			break
		}

		resultMap := make(map[string]interface{})
		for _, dc := range deviceCodes {
			// Verify product code
			dev, err := store.GetDevice(dc)
			if err != nil || dev.ProductCode != productCode {
				resultMap[dc] = map[string]interface{}{
					"code":    400,
					"message": "Device not found or product code mismatch",
				}
				continue
			}

			_, err = p.Ctx.IssueCommand(dc, "set_properties", setParams)
			if err != nil {
				resultMap[dc] = map[string]interface{}{
					"code":    500,
					"message": err.Error(),
				}
			} else {
				resultMap[dc] = map[string]interface{}{
					"code":    200,
					"message": "success",
				}
			}
		}
		reply.Data = resultMap

	case "service_batch_invoke":
		// 同产品多设备的服务控制下发接口
		productCode := cmd.ProductCode
		if productCode == "" {
			reply.Code = 400
			reply.Message = "productCode is required"
			break
		}

		params, _ := cmd.Params.(map[string]interface{})
		serviceCode, _ := params["service_id"].(string)
		invokeParams, _ := params["params"].(map[string]interface{})

		var deviceCodes []string
		if dcs, ok := params["device_codes"].([]interface{}); ok {
			for _, dc := range dcs {
				if s, ok := dc.(string); ok {
					deviceCodes = append(deviceCodes, s)
				}
			}
		}

		if len(deviceCodes) == 0 {
			reply.Code = 400
			reply.Message = "device_codes is required and cannot be empty"
			break
		}
		if serviceCode == "" {
			reply.Code = 400
			reply.Message = "service_id is required"
			break
		}

		resultMap := make(map[string]interface{})
		for _, dc := range deviceCodes {
			// Verify product code
			dev, err := store.GetDevice(dc)
			if err != nil || dev.ProductCode != productCode {
				resultMap[dc] = map[string]interface{}{
					"code":    400,
					"message": "Device not found or product code mismatch",
				}
				continue
			}

			res, err := p.Ctx.IssueCommand(dc, serviceCode, invokeParams)
			if err != nil {
				resultMap[dc] = map[string]interface{}{
					"code":    500,
					"message": err.Error(),
				}
			} else {
				resultMap[dc] = map[string]interface{}{
					"code":    200,
					"message": "success",
					"data":    res,
				}
			}
		}
		reply.Data = resultMap

	default:
		reply.Code = 404
		reply.Message = "Service not found"
	}

	replyTopic := topic + "_reply"
	if p.client != nil && p.client.IsConnected() {
		replyData, _ := json.Marshal(reply)
		p.client.Publish(replyTopic, 0, false, replyData)
	}
}
