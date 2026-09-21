package bacnet

import (
 "fmt"
 "strconv"
 "strings"
)

// normalizeDeviceConfig accepts both the native schema and exported simulator
// profiles. Work on copies so runtime normalization never mutates saved metadata.
func normalizeDeviceConfig(src map[string]interface{}) (map[string]interface{}, error) {
 out:=make(map[string]interface{},len(src))
 for k,v:=range src { out[k]=v }
 alias:=func(m map[string]interface{}, key string, aliases ...string) {
  if _,ok:=m[key]; ok { return }; for _,a:=range aliases { if v,ok:=m[a];ok { m[key]=v; return } }
 }
 alias(out,"ip","host")
 alias(out,"device_id","device_instance")
 objectTypes:=map[string]int{"analog_input":0,"analog_output":1,"analog_value":2,"binary_input":3,"binary_output":4,"binary_value":5,"device":8,"multistate_input":13,"multi_state_input":13,"multistate_output":14,"multi_state_output":14,"multistate_value":19,"multi_state_value":19}
 points,ok:=src["points"].([]interface{}); if !ok { return out,nil }
 normalized:=make([]interface{},0,len(points))
 for _,raw:=range points {
  original,ok:=raw.(map[string]interface{}); if !ok { return nil,fmt.Errorf("invalid BACnet point config") }
  pt:=make(map[string]interface{},len(original)); for k,v:=range original { pt[k]=v }
  alias(pt,"object_type","block"); alias(pt,"instance_id","instance","address")
  for _,key:=range []string{"object_type","instance_id"} {
   value,ok:=pt[key]; if !ok { return nil,fmt.Errorf("BACnet point %v: missing %s",pt["name"],key) }
   s:=fmt.Sprint(value)
   if key=="object_type" { if n,ok:=objectTypes[strings.ToLower(strings.TrimSpace(s))];ok { s=strconv.Itoa(n) } }
   n,err:=strconv.ParseUint(s,10,32)
   limit:=uint64(4194303); if key=="object_type" { limit=1023 }
   if err!=nil || n>limit { return nil,fmt.Errorf("BACnet point %v: invalid %s %v",pt["name"],key,value) }
   pt[key]=float64(n)
  }
  normalized=append(normalized,pt)
 }
 out["points"]=normalized
 return out,nil
}
