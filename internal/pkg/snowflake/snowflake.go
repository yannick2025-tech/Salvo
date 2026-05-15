// Package snowflake implements a distributed, Twitter-style Snowflake ID generator.
//
// Each ID is a 64-bit integer composed of:
//   - 1 bit sign (unused)
//   - 41 bits timestamp (milliseconds since custom epoch 2024-01-01)
//   - 10 bits node ID (0–1023)
//   - 12 bits sequence (0–4095 per millisecond per node)
//
// IDs are JSON-serialised as strings to avoid JavaScript float precision loss.
package snowflake

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

const (
	// nodeBits is the number of bits allocated for the node ID.
	nodeBits uint8 = 10
	// stepBits is the number of bits allocated for the sequence counter.
	stepBits uint8 = 12
	// nodeMax is the maximum value of a node ID.
	nodeMax int64 = -1 ^ (-1 << nodeBits)
	// stepMask is the bitmask for the sequence counter.
	stepMask int64 = -1 ^ (-1 << stepBits)
	// timeShift is the bit offset for the timestamp portion.
	timeShift uint8 = nodeBits + stepBits
	// nodeShift is the bit offset for the node ID portion.
	nodeShift uint8 = stepBits

	// customEpoch is the millisecond timestamp for 2024-01-01 00:00:00 UTC.
	customEpoch int64 = 1704067200000
)

// ID represents a Snowflake identifier stored as a 64-bit integer.
// It marshals to JSON as a string to prevent precision loss in JavaScript.
type ID int64

// Node is a Snowflake ID generator bound to a specific node ID.
// It is safe for concurrent use; the mutex serialises ID generation
// within the same millisecond.
type Node struct {
	mu       sync.Mutex
	nodeID   int64
	step     int64
	lastTime int64
}

// NewNode creates a new Snowflake ID generator for the given node ID.
// The nodeID must be in the range [0, 1023]; otherwise an error is returned.
func NewNode(nodeID int64) (*Node, error) {
	if nodeID < 0 || nodeID > nodeMax {
		return nil, fmt.Errorf("node ID must be between 0 and %d", nodeMax)
	}
	return &Node{
		nodeID:   nodeID,
		lastTime: 0,
		step:     0,
	}, nil
}

// Generate produces a new unique Snowflake ID.
// It blocks only when the sequence counter overflows within the same
// millisecond, waiting until the next millisecond.
func (n *Node) Generate() ID {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now().UnixMilli()

	if now == n.lastTime {
		n.step = (n.step + 1) & stepMask
		if n.step == 0 {
			for now <= n.lastTime {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		n.step = 0
	}

	n.lastTime = now

	id := ID((now-customEpoch)<<timeShift | n.nodeID<<nodeShift | n.step)
	return id
}

// Int64 returns the ID as a plain int64.
func (id ID) Int64() int64 {
	return int64(id)
}

// String returns the decimal string representation of the ID.
func (id ID) String() string {
	return strconv.FormatInt(int64(id), 10)
}

// Time returns the millisecond timestamp embedded in the ID (absolute, not epoch-relative).
func (id ID) Time() int64 {
	return (int64(id) >> timeShift) + customEpoch
}

// NodeID returns the node ID embedded in the ID.
func (id ID) NodeID() int64 {
	return (int64(id) >> nodeShift) & nodeMax
}

// Sequence returns the sequence number embedded in the ID.
func (id ID) Sequence() int64 {
	return int64(id) & stepMask
}

// MarshalJSON implements json.Marshaler.
// It serialises the ID as a JSON string to avoid float precision loss.
func (id ID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
// It accepts both JSON string and number formats.
func (id *ID) UnmarshalJSON(data []byte) error {
	str := string(data)
	if str == "null" {
		return nil
	}
	// Try to unmarshal as string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		val, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid snowflake ID %q: %w", s, err)
		}
		*id = ID(val)
		return nil
	}
	// If string unmarshal fails, try to unmarshal as number
	var val int64
	if err := json.Unmarshal(data, &val); err != nil {
		return fmt.Errorf("snowflake ID must be a string or number, got %s", string(data))
	}
	*id = ID(val)
	return nil
}

// Parse sets the ID from a decimal string representation.
// Returns an error for empty, non-numeric, or negative values.
func (id *ID) Parse(s string) error {
	if s == "" {
		return fmt.Errorf("empty snowflake ID string")
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil || val < 0 {
		return fmt.Errorf("invalid snowflake ID %q", s)
	}
	*id = ID(val)
	return nil
}
