package modbus

import "noyo/core"

// ProductConfigSchema defines the schema for Modbus product configuration
const ProductConfigSchema = `
{
  "type": "object",
  "properties": {
    "polling_groups": {
      "type": "array",
      "title": "Data Polling Groups",
      "items": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string",
            "title": "Group Name",
            "minLength": 1
          },
          "slave_id": {
            "type": "integer",
            "title": "Slave ID",
            "default": 1,
            "minimum": 0,
            "maximum": 247
          },
          "function_code": {
            "type": "integer",
            "title": "Function Code",
            "enum": [1, 2, 3, 4],
            "enumNames": ["Read Coils (01)", "Read Discrete Inputs (02)", "Read Holding Registers (03)", "Read Input Registers (04)"],
            "default": 3
          },
          "start_address": {
            "type": "integer",
            "title": "Start Address",
            "minimum": 0,
            "maximum": 65535
          },
          "length": {
            "type": "integer",
            "title": "Length (Count)",
            "minimum": 1,
            "maximum": 125,
            "default": 10
          },
          "interval": {
            "type": "string",
            "title": "Polling Interval",
            "default": "@every 1s",
            "pattern": "^@every \\d+(ms|s|m|h)$",
            "description": "Format: @every 1s, @every 500ms"
          }
        },
        "required": ["name", "slave_id", "function_code", "start_address", "length"]
      }
    },
    "events": {
      "type": "array",
      "title": "Event Rules",
      "items": {
        "type": "object",
        "properties": {
          "identifier": {
            "type": "string",
            "title": "Event Identifier",
            "description": "Must match the event identifier in Thing Model"
          },
          "name": {
            "type": "string",
            "title": "Event Name"
          },
          "trigger_logic": {
            "type": "string",
            "title": "Trigger Logic",
            "enum": ["or", "and"],
            "enumNames": ["Satisfy ANY (OR)", "Satisfy ALL (AND)"],
            "default": "or"
          },
          "triggers": {
             "type": "array",
             "title": "Trigger Conditions",
             "items": {
                "type": "object",
                "title": "Trigger",
                "properties": {
                  "point": {
                    "type": "string",
                    "title": "Source Point",
                    "description": "Select the point to monitor"
                  },
                  "operator": {
                    "type": "string",
                    "title": "Operator",
                    "enum": [">", "<", "==", "!=", ">=", "<=", "change"],
                    "enumNames": ["Greater Than", "Less Than", "Equal", "Not Equal", "Greater or Equal", "Less or Equal", "On Change"],
                    "default": ">"
                  },
                  "value": {
                    "type": "number",
                    "title": "Threshold Value"
                  },
                  "debounce": {
                    "type": "integer",
                    "title": "Debounce (ms)",
                    "default": 0,
                    "description": "Duration the condition must be met before triggering"
                  }
                },
                "required": ["point", "operator"]
             }
          },
          "condition_logic": {
            "type": "string",
            "title": "Condition Logic",
            "enum": ["and", "or"],
            "enumNames": ["Satisfy ALL (AND)", "Satisfy ANY (OR)"],
            "default": "and"
          },
          "conditions": {
            "type": "array",
            "title": "Pre-conditions",
            "items": {
                "type": "object",
                "properties": {
                  "point": {
                    "type": "string",
                    "title": "Point Name"
                  },
                  "operator": {
                    "type": "string",
                    "title": "Operator",
                    "enum": [">", "<", "==", "!=", ">=", "<=", "change"],
                    "enumNames": ["Greater Than", "Less Than", "Equal", "Not Equal", "Greater or Equal", "Less or Equal", "On Change"],
                    "default": ">"
                  },
                  "value": {
                    "type": "number",
                    "title": "Threshold Value"
                  }
                },
                "required": ["point", "operator"]
             }
          },
          "report_interval": {
            "type": "integer",
            "title": "Report Interval (Seconds)",
            "description": "-1: Rising Edge Only (Once), 0: Always Report (Every Cycle), >0: Throttle (Every N Seconds)",
            "default": -1
          },
          "params": {
            "type": "object",
            "title": "Output Parameters",
            "description": "Map Event Parameters (Key) to Data Points (Value)",
            "additionalProperties": {
              "type": "string",
              "title": "Source Point Name"
            }
          }
        },
        "required": ["identifier"]
      }
    }
  }
}
`

// DeviceConfigSchema defines the schema for Modbus device configuration (TCP/RTU)
const DeviceConfigSchema = `
{
  "type": "object",
  "properties": {
    "ip": {
      "type": "string",
      "title": "IP Address",
      "format": "ipv4",
      "default": "127.0.0.1"
    },
    "port": {
      "type": "integer",
      "title": "Port",
      "default": 502,
      "minimum": 1,
      "maximum": 65535
    },
    "protocol_type": {
      "type": "string",
      "title": "Protocol Type",
      "enum": ["Modbus-TCP", "Modbus-RTU over TCP"],
      
      "default": "Modbus-TCP"
    },
    "timeout": {
      "type": "integer",
      "title": "Timeout (ms)",
      "default": 2000,
      "minimum": 100
    },
    "max_group_length": {
      "type": "integer",
      "title": "Max Group Length",
      "description": "Maximum number of registers per request",
      "default": 120,
      "minimum": 1,
      "maximum": 125
    },
    "max_address_gap": {
      "type": "integer",
      "title": "Max Address Gap",
      "description": "Maximum address gap to merge into one request",
      "default": 20,
      "minimum": 0
    }
  },
  "required": ["ip", "port", "protocol_type"]
}
`

// SubDeviceConfigSchema defines the schema for Modbus sub-device configuration
// Since sub-devices are logical and reuse the gateway's connection,
// and polling groups define the slave ID, we don't strictly need configuration here.
const SubDeviceConfigSchema = `
{
  "type": "object",
  "properties": {
    "description": {
      "type": "string",
      "title": "Description",
      "description": "Optional description for this sub-device"
    }
  }
}
`

// PointConfigSchema defines the schema for Modbus point mapping
const PointConfigSchema = `
{
  "type": "object",
  "properties": {
    "polling_group": {
      "type": "string",
      "title": "Polling Group",
      "description": "Select the polling group defined in Product Config"
    },
    "interval": {
      "type": "integer",
      "title": "Collection Interval (ms)",
      "minimum": 100,
      "description": "Optional. Overrides default polling interval."
    },
    "offset": {
      "type": "integer",
      "title": "Address Offset",
      "minimum": 0,
      "description": "Offset from the group's Start Address"
    },
    "data_type": {
      "type": "string",
      "title": "Data Type",
      "enum": ["int16", "uint16", "int32", "uint32", "float32", "bool"],
      "default": "int16"
    },
    "byte_order": {
      "type": "string",
      "title": "Byte Order",
      "enum": ["ABCD", "BADC", "CDAB", "DCBA"],
      "default": "ABCD",
      "description": "ABCD=BigEndian, DCBA=LittleEndian, etc."
    },
    "read_expr": {
      "type": "string",
      "title": "Read Expression",
      "description": "Expression to transform raw value. Variable: 'x'. Example: x * 0.1"
    },
    "write_expr": {
      "type": "string",
      "title": "Write Expression",
      "description": "Expression to transform value before write. Variable: 'x'. Example: x * 10"
    },
    "is_property": {
      "type": "boolean",
      "title": "Report as Property",
      "default": true,
      "description": "If true, this point is reported as a property. If false, it is only used internally (e.g. for events)."
    },
    "enable_write": {
      "type": "boolean",
      "title": "Enable Write/Control",
      "default": false
    },
    "write_mode": {
      "type": "string",
      "title": "Write Mode",
      "enum": ["same_as_read", "custom"],
      "enumNames": ["Same as Read (Same Register)", "Custom (Different Register)"],
      "default": "same_as_read"
    },
    "write_address": {
      "type": "integer",
      "title": "Write Absolute Address",
      "minimum": 0,
      "description": "Absolute Modbus Address (e.g. 40001)"
    },
    "write_function_code": {
      "type": "integer",
      "title": "Write Function Code",
      "enum": [5, 6, 15, 16],
      "enumNames": ["Write Single Coil (05)", "Write Single Register (06)", "Write Multiple Coils (15)", "Write Multiple Registers (16)"]
    },
    "write_data_type": {
      "type": "string",
      "title": "Write Data Type",
      "enum": ["int16", "uint16", "int32", "uint32", "float32", "bool"],
      "description": "If different from Read Data Type"
    },
    "write_slave_id": {
        "type": "integer",
        "title": "Write Slave ID",
        "minimum": 0,
        "maximum": 247,
        "description": "Optional. Defaults to Polling Group's Slave ID."
    }
  },
  "required": ["polling_group", "offset", "data_type"],
  "dependencies": {
    "write_mode": ["enable_write"],
    "write_address": ["enable_write"],
    "write_function_code": ["enable_write"],
    "write_data_type": ["enable_write"],
    "write_slave_id": ["enable_write"]
  }
}
`

func (p *ModbusPlugin) GetProductConfigSchema() ([]byte, error) {
	return nil, nil
}

func (p *ModbusPlugin) GetDeviceConfigSchema(meta core.DeviceMeta) ([]byte, error) {
	if meta.ParentCode != "" {
		return []byte(SubDeviceConfigSchema), nil
	}
	return []byte(DeviceConfigSchema), nil
}

func (p *ModbusPlugin) GetPointConfigSchema() ([]byte, error) {
	return []byte(PointConfigSchema), nil
}

// SubDeviceConfigCustomizable Modbus 子设备配置固定（slave_id 等），不允许用户自定义
func (p *ModbusPlugin) SubDeviceConfigCustomizable() bool {
	return false
}
