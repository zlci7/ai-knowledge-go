package idgen

import (
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	node *snowflake.Node
	once sync.Once
)

// InitSnowflake 初始化雪花算法节点
// nodeId: 机器ID，范围 0-1023（10位）
func InitSnowflake(nodeId int64) error {
	var err error
	once.Do(func() {
		node, err = snowflake.NewNode(nodeId)
		if err != nil {
		}
	})
	return err
}

// GenInt64ID 生成 int64 类型的 ID
func GenInt64ID() int64 {
	if node == nil {
		return 0
	}
	return node.Generate().Int64()
}

// GenStringID 生成字符串类型的 ID
func GenStringID() string {
	if node == nil {
		return ""
	}
	return node.Generate().String()
}
