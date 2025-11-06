package streams

import (
	"testing"

	"github.com/openrelayxyz/cardinal-streams/v2/delivery"
	types "github.com/openrelayxyz/cardinal-types"
)

func TestValidateBatchHasStateUpdates(t *testing.T) {
    tests := []struct{
        name string
        batch *delivery.PendingBatch
        wantErr bool
    }{
        {
            name: "missing state updates",
            batch: &delivery.PendingBatch{
                Number: 100,
                Hash: types.Hash{0x01},
                Values: map[string][]byte{
                    "c/1/b/abc123/h": []byte("header"),
                    "c/1/n/64": []byte("block_num"),
                },
            },
            wantErr: true,
        },
        {
            name: "account state updates returns no error",
            batch: &delivery.PendingBatch{
                Number: 100,
                Hash: types.Hash{0x01},
                Values: map[string][]byte{
                    "c/1/b/abc123/h": []byte("header"),
                    "c/1/a/def456/d": []byte("account"),
                },
            },
            wantErr: false,
        },
        {
            name: "with storage state updates returns no error",
            batch: &delivery.PendingBatch{
                Number: 100,
                Hash: types.Hash{0x01},
                Values: map[string][]byte{
                    "c/1/b/abc123/h": []byte("header"),
                    "c/1/a/def456/s/123": []byte("storage"),
                },
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        err := validatePendingBatches(tt.batch)
            
        if tt.wantErr && err == nil {
            t.Error("Expected error but got none")
        }
        if !tt.wantErr && err != nil {
            t.Errorf("Unexpected error: %v", err)
        }
    }
}