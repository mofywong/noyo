package pool

import (
	"noyo/core/platform"
	"sync"
)

// DataModelPool manages a pool of DataModel objects to reduce GC pressure
var DataModelPool = &dataModelPool{
	pool: sync.Pool{
		New: func() interface{} {
			return &platform.DataModel{
				Payload: make(map[string]interface{}),
			}
		},
	},
}

type dataModelPool struct {
	pool sync.Pool
}

// Get returns a DataModel from the pool
func (p *dataModelPool) Get() *platform.DataModel {
	return p.pool.Get().(*platform.DataModel)
}

// Put returns a DataModel to the pool
func (p *dataModelPool) Put(d *platform.DataModel) {
	// Clear fields
	d.DeviceCode = ""
	d.ProductCode = ""
	d.Type = ""
	d.UniqueId = ""
	d.Timestamp = 0

	// Clear map (Go 1.21+)
	// If the map is too large, drop it?
	if len(d.Payload) > 100 {
		d.Payload = make(map[string]interface{})
	} else {
		for k := range d.Payload {
			delete(d.Payload, k)
		}
	}

	p.pool.Put(d)
}
