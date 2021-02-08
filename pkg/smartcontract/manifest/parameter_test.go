package manifest

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestParameter_ToStackItemFromStackItem(t *testing.T) {
	p := &Parameter{
		Name: "param",
		Type: smartcontract.StringType,
	}
	expected := stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray([]byte(p.Name)),
		stackitem.NewBigInteger(big.NewInt(int64(p.Type))),
	})
	CheckToFromStackItem(t, p, expected)
}

func TestParameter_FromStackItemErrors(t *testing.T) {
	errCases := map[string]stackitem.Item{
		"not a struct":       stackitem.NewArray([]stackitem.Item{}),
		"invalid length":     stackitem.NewStruct([]stackitem.Item{}),
		"invalid name type":  stackitem.NewStruct([]stackitem.Item{stackitem.NewInterop(nil), stackitem.Null{}}),
		"invalid type type":  stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.Null{}}),
		"invalid type value": stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.NewBigInteger(big.NewInt(-100500))}),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			p := new(Parameter)
			require.Error(t, p.FromStackItem(errCase))
		})
	}
}
