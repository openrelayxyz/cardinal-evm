package tracers


import (
	"encoding/json"
	"fmt"
	"errors"
)

// Config are the configuration options for structured logger the EVM
type LoggerConfig struct {
	EnableMemory     bool // enable memory capture
	DisableStack     bool // disable stack capture
	DisableStorage   bool // disable storage capture
	EnableReturnData bool // enable return data capture
	Debug            bool // print output during capture end
	Limit            int  // maximum length of output, but zero means unlimited
	// Chain overrides, can be used to execute a trace using future fork rules
	Overrides interface{} `json:"overrides,omitempty"`
}

// TraceConfig holds extra parameters to trace functions.
type TraceConfig struct {
	*LoggerConfig
	Tracer  *string
	Timeout *string
	Reexec  *uint64
	// Config specific to given tracer. Note struct logger
	// config are historically embedded in main object.
	TracerConfig json.RawMessage
}

// TraceCallConfig is the config for traceCall API. It holds one more
// field to override the state for tracing.
type TraceCallConfig struct {
	TraceConfig
	StateOverrides interface{}
	BlockOverrides interface{}
}

const (
	memoryPadLimit = 1024 * 1024
)

// GetMemoryCopyPadded returns offset + size as a new slice.
// It zero-pads the slice if it extends beyond memory bounds.
func GetMemoryCopyPadded(m []byte, offset, size int64) ([]byte, error) {
	if offset < 0 || size < 0 {
		return nil, errors.New("offset or size must not be negative")
	}
	length := int64(len(m))
	if offset+size < length { // slice fully inside memory
		return memoryCopy(m, offset, size), nil
	}
	paddingNeeded := offset + size - length
	if paddingNeeded > memoryPadLimit {
		return nil, fmt.Errorf("reached limit for padding memory slice: %d", paddingNeeded)
	}
	cpy := make([]byte, size)
	if overlap := length - offset; overlap > 0 {
		copy(cpy, MemoryPtr(m, offset, overlap))
	}
	return cpy, nil
}

func memoryCopy(m []byte, offset, size int64) (cpy []byte) {
	if size == 0 {
		return nil
	}

	if len(m) > int(offset) {
		cpy = make([]byte, size)
		copy(cpy, m[offset:offset+size])

		return
	}

	return
}

// MemoryPtr returns a pointer to a slice of memory.
func MemoryPtr(m []byte, offset, size int64) []byte {
	if size == 0 {
		return nil
	}

	if len(m) > int(offset) {
		return m[offset : offset+size]
	}

	return nil
}