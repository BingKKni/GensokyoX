// Package interactionwait 提供 webhook 模式下 INTERACTION_CREATE 事件的"应用端覆盖"机制。
//
// 当 webhook 收到一次按钮回调事件时，框架可以为该 interaction_id 注册一个 pending 槽位，
// 然后在最长 d 时间内等待应用端通过 HTTP API（send_group_msg / send_private_msg /
// put_interaction）提供自定义 code。一旦槽位被填充，webhook 即可用该 code 响应平台
// （响应体形如 {"op":12,"code":N}），从而覆盖默认的"操作成功"提示。
//
// 没有人填充时，wait 函数会在超时后返回 (0, false)，由 webhook 端按照 webhook_resp_code
// 配置项决定最终响应。
package interactionwait

import (
	"sync"
	"time"
)

// slot 表示一个待回应的 interaction 槽位。
type slot struct {
	ch chan int
}

var (
	mu      sync.Mutex
	pending = make(map[string]*slot)
)

// Register 为 interactionID 注册一个待回应槽位，并返回一个 wait 函数。
//
// wait(d) 阻塞至多 d 时长：
//   - 期间若 TryFill 投递了 code，返回 (code, true)；
//   - 时长 d 已过且无人投递，返回 (0, false)。
//
// 无论返回哪种情况，wait 返回后槽位一定从全局 map 中移除（不会泄漏）。
//
// 同一 interactionID 重复 Register 会覆盖旧槽位（旧槽位的 wait 仍能通过 timer 自然清理）。
func Register(interactionID string) func(time.Duration) (int, bool) {
	s := &slot{ch: make(chan int, 1)}
	mu.Lock()
	pending[interactionID] = s
	mu.Unlock()

	return func(d time.Duration) (int, bool) {
		// 不等待时，立即从 map 中清理自身，返回未填充
		if d <= 0 {
			mu.Lock()
			if pending[interactionID] == s {
				delete(pending, interactionID)
			}
			mu.Unlock()
			return 0, false
		}

		timer := time.NewTimer(d)
		defer timer.Stop()

		select {
		case code := <-s.ch:
			// TryFill 路径已经从 map 删除了 slot，无需再做
			return code, true

		case <-timer.C:
			// 超时：尝试自己摘除 slot
			mu.Lock()
			owned := pending[interactionID] == s
			if owned {
				delete(pending, interactionID)
			}
			mu.Unlock()

			if !owned {
				// 极端竞态：超时与 TryFill 同时发生，TryFill 已抢到 slot 但还没把 code
				// 送进 channel。等它送达，避免应用端误以为 webhook 用了它的 code。
				code := <-s.ch
				return code, true
			}
			return 0, false
		}
	}
}

// TryFill 尝试为 interactionID 投递 code。
//
// 返回值：
//   - true  : 槽位存在且 code 已成功投递 → 调用方应跳过实际的 PUT API 调用，
//             因为 webhook 端会用此 code 响应平台。
//   - false : 槽位不存在（未注册 / 已超时 / 已被消费）→ 调用方按原逻辑走 PUT。
//
// code 的合法取值 0~5，本函数不做校验，由调用方保证。
func TryFill(interactionID string, code int) bool {
	mu.Lock()
	s, ok := pending[interactionID]
	if ok {
		delete(pending, interactionID)
	}
	mu.Unlock()
	if !ok {
		return false
	}
	// channel 容量为 1，单次投递必然成功且非阻塞
	s.ch <- code
	return true
}
