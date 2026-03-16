package redis

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var errConversationBusy = errors.New("conversation is busy, please retry")

const unlockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
else
  return 0
end
`

// AcquireConversationLock acquires a short-lived lock for a conversation.
// It retries for a short time to reduce user-facing contention.
func AcquireConversationLock(ctx context.Context, convID string, ttl time.Duration) (func(), error) {
	lockKey := fmt.Sprintf("stm:%s:lock", convID)
	token := fmt.Sprintf("%d", time.Now().UnixNano())

	deadline := time.Now().Add(2 * time.Second)
	for {
		ok, err := Rdb.SetNX(ctx, lockKey, token, ttl).Result()
		if err != nil {
			return nil, err
		}
		if ok {
			unlock := func() {
				_, _ = Rdb.Eval(context.Background(), unlockScript, []string{lockKey}, token).Result()
			}
			return unlock, nil
		}

		if time.Now().After(deadline) {
			return nil, errConversationBusy
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
