package builtin

import (
	"sync"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

var (
	snowflakeNode     *snowflake.Node
	snowflakeNodeOnce sync.Once
)

// SnowflakeId implements the __snowflakeId system function.
// It generates a unique, monotonically increasing 64-bit Snowflake ID
// and returns it as a 19-digit decimal string.
func SnowflakeId() (string, error) {
	snowflakeNodeOnce.Do(func() {
		var err error
		snowflakeNode, err = snowflake.NewNode(0)
		if err != nil {
			// This should never happen with nodeID=0 (valid range 0-1023).
			panic("snowflake: failed to create node: " + err.Error())
		}
	})
	return snowflakeNode.Generate().String(), nil
}