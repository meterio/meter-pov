package script

import (
        "errors"
        "fmt"
        "sync"
)

// Registry is the hub of all modules on the chain
type Module interface {
        //msgHdler
}

type Registry struct {
        Modules sync.Map
}

func (r *Registry) Register(id string, p Module) error {
        _, loaded := r.Modules.LoadOrStore(id, p)
        if loaded {
                return errors.New(fmt.Sprintf("Module with ID %s is already registered", id))
        }
        return nil
}

// ForceRegister registers with a unique ID and force replacing the previous module if it exists
func (r *Registry) ForceRegister(id string, p Module) error {
        r.Modules.Store(id, p)
        return nil
}

// Find  by ID
func (r *Registry) Find(id string) (Module, bool) {
        value, ok := r.Modules.Load(id)
        if !ok {
                return nil, false
        }
        p, ok := value.(Module)
        if !ok {
                panic("Registry stores the item which is not a Module")
        }
        return p, true
}

func (r *Registry) All() []Module {
        all := make([]Module, 0)
        r.Modules.Range(func(_, value interface{}) bool {
                p, ok := value.(Module)
                if !ok {
                        panic("Registry stores the item which is not a module")
                }
                all = append(all, p)
                return true
        })
        return all
}
