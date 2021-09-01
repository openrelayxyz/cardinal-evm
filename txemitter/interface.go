package txemitter

import (
	"github.com/openrelayxyz/cardinal-evm/types"
)

type TransactionProducer interface {
  Emit(marshall(*types.Transaction)) error
  Close()
}
