package access

import (
	"fmt"
)

// Error 业务错误 | 对应 TS 的 RerouteMoneyException：HTTP 可能是 200, 但信封 code 非 OK
//
// 调用方用 `errors.As(err, new(*access.Error))` 取 Code 做分支。
type Error struct {
	// Code 错误码 | 如 SIGNATURE_INVALID、ORDER_IDEMPOTENCY_CONFLICT
	Code string
	// Message 错误信息
	Message string
}

// Error 实现 error
func (e *Error) Error() string {
	return fmt.Sprintf("access: 业务错误 [%s] %s", e.Code, e.Message)
}

// HTTPStatusError 传输层错误 | 响应体不是合法信封（网关 5xx、HTML 错误页等）时返回
type HTTPStatusError struct {
	// StatusCode HTTP 状态码
	StatusCode int
	// Body 响应体片段 | 最多 512 字节, 便于排查网关侧问题
	Body string
}

// Error 实现 error
func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("access: HTTP %d, 响应体不是合法信封: %s", e.StatusCode, e.Body)
}
