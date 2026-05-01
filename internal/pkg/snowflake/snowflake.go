package snowflake

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

const (
	nodeBits  uint8 = 10
	stepBits  uint8 = 12
	nodeMax   int64 = -1 ^ (-1 << nodeBits)
	stepMask  int64 = -1 ^ (-1 << stepBits)
	timeShift uint8 = nodeBits + stepBits
	nodeShift uint8 = stepBits

	customEpoch int64 = 1704067200000
)

type ID int64

type Node struct {
	mu        sync.Mutex
	nodeID    int64
	step      int64
	lastTime  int64
}

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

func (id ID) Int64() int64 {
	return int64(id)
}

func (id ID) String() string {
	return strconv.FormatInt(int64(id), 10)
}

func (id ID) Time() int64 {
	return (int64(id) >> timeShift) + customEpoch
}

func (id ID) NodeID() int64 {
	return (int64(id) >> nodeShift) & nodeMax
}

func (id ID) Sequence() int64 {
	return int64(id) & stepMask
}

func (id ID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

func (id *ID) UnmarshalJSON(data []byte) error {
	str := string(data)
	if str == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("snowflake ID must be a string, got %s", string(data))
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid snowflake ID %q: %w", s, err)
	}
	*id = ID(val)
	return nil
}

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
