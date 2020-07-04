// Copyright (c) 2020 The Meter.io developerslopers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package script

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
)

// Registry is the hub of all modules on the chain
type Module struct {
	modName    string
	modID      uint32
	modHandler func(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (ret []byte, leftOverGas uint64, err error)
}

func (m *Module) ToString() string {
	return fmt.Sprintf("Module::: Name: %v, ID: %v", m.modName, m.modID)
}

type Registry struct {
	Modules sync.Map
}

func (r *Registry) Register(modID uint32, p *Module) error {
	_, loaded := r.Modules.LoadOrStore(modID, *p)
	if loaded {
		return errors.New(fmt.Sprintf("Module with ID %v is already registered", modID))
	}
	return nil
}

// ForceRegister registers with a unique ID and force replacing the previous module if it exists
func (r *Registry) ForceRegister(modID uint32, p *Module) error {
	r.Modules.Store(modID, *p)
	return nil
}

// Find by ID
func (r *Registry) Find(modID uint32) (*Module, bool) {
	value, ok := r.Modules.Load(modID)
	if !ok {
		return nil, false
	}
	p, ok := value.(Module)
	if !ok {
		panic("Registry stores the item which is not a Module")
	}
	return &p, true
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
