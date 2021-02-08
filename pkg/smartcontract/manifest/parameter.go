package manifest

import (
	"errors"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Parameter represents smartcontract's parameter's definition.
type Parameter struct {
	Name string                  `json:"name"`
	Type smartcontract.ParamType `json:"type"`
}

// NewParameter returns new parameter of specified name and type.
func NewParameter(name string, typ smartcontract.ParamType) Parameter {
	return Parameter{
		Name: name,
		Type: typ,
	}
}

// ToStackItem converts Parameter to stackitem.Item.
func (p *Parameter) ToStackItem() stackitem.Item {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(p.Name),
		stackitem.Make(int(p.Type)),
	})
}

// FromStackItem converts stackitem.Item to Parameter.
func (p *Parameter) FromStackItem(item stackitem.Item) error {
	var err error
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Parameter stackitem type")
	}
	param := item.Value().([]stackitem.Item)
	if len(param) != 2 {
		return errors.New("invalid Parameter stackitem length")
	}
	p.Name, err = stackitem.ToString(param[0])
	if err != nil {
		return err
	}
	typ, err := param[1].TryInteger()
	if err != nil {
		return err
	}
	p.Type, err = smartcontract.ConvertToParamType(int(typ.Int64()))
	if err != nil {
		return err
	}
	return nil
}
