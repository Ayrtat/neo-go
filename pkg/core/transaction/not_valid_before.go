package transaction

import (
	"encoding/binary"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// NotValidBefore represents attribute with the height transaction is not valid before.
type NotValidBefore struct {
	Height uint32 `json:"height"`
}

// DecodeBinary implements io.Serializable interface.
func (n *NotValidBefore) DecodeBinary(br *io.BinReader) {
	bytes := br.ReadVarBytes(4)
	n.Height = binary.LittleEndian.Uint32(bytes)
}

// EncodeBinary implements io.Serializable interface.
func (n *NotValidBefore) EncodeBinary(w *io.BinWriter) {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, n.Height)
	w.WriteVarBytes(bytes)
}

func (n *NotValidBefore) toJSONMap(m map[string]interface{}) {
	m["height"] = n.Height
}
