package article

import "sync"

var (
	obs      *observer
	obsMutex sync.Mutex
)

type observer struct {
	ProcessorMap sync.Map `json:"processor_map"`
}

// GetObs ...
func GetObs() *observer {
	obsMutex.Lock()
	if obs == nil {
		obs = &observer{
			ProcessorMap: sync.Map{},
		}
	}
	obsMutex.Unlock()
	return obs
}

// AddProcessor ...
func (o *observer) AddProcessor(p processor) {
	obsMutex.Lock()
	o.ProcessorMap.Store(p.GetID(), p)
	obsMutex.Unlock()
}

// DeleteProcessor ...
func (o *observer) DeleteProcessor(id int64) {
	obsMutex.Lock()
	o.ProcessorMap.Delete(id)
	obsMutex.Unlock()
}

// PostEvent ...
func (o *observer) PostEvent(e Event) error {
	if e.ID == 0 {
		return nil
	}
	if e.Type == 0 {
		return nil
	}

	o.ProcessorMap.Range(
		func(k, v interface{}) bool {
			p := v.(processor)
			if e.Type == TypeEventAdd {
				if err := p.EntryAdded(e); err != nil {
					return false
				}
			}
			if e.Type == TypeEventDelete {
				if err := p.EntryDeleted(e); err != nil {
					return false
				}
			}
			if e.Type == TypeEventModify {
				if err := p.EntryModified(e); err != nil {
					return false
				}
			}
			return true
		})

	return nil
}
