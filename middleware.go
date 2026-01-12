// Copyright 2025 zampo.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// @contact  zampo3380@gmail.com

package xxljob

import (
	"context"
	"fmt"
	"time"
)

// Chain 中间件链
// 按顺序执行多个中间件
func Chain(middlewares ...Middleware) Middleware {
	return func(next TaskHandler) TaskHandler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// applyMiddlewares 应用中间件链到任务处理器
func applyMiddlewares(handler TaskHandler, middlewares []Middleware) TaskHandler {
	if len(middlewares) == 0 {
		return handler
	}

	// 构建中间件链
	chain := Chain(middlewares...)
	return chain(handler)
}

// RecoveryMiddleware 恢复中间件
// 捕获 panic 并转换为 error
func RecoveryMiddleware() Middleware {
	return func(next TaskHandler) TaskHandler {
		return func(ctx context.Context, param string) (err error) {
			defer func() {
				if r := recover(); r != nil {
					if e, ok := r.(error); ok {
						err = e
					} else {
						err = &panicError{value: r}
					}
				}
			}()
			return next(ctx, param)
		}
	}
}

// panicError panic 错误类型
type panicError struct {
	value interface{}
}

func (e *panicError) Error() string {
	return fmt.Sprintf("panic: %v", e.value)
}

// TimeoutMiddleware 超时中间件
// 为任务执行设置超时时间
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next TaskHandler) TaskHandler {
		return func(ctx context.Context, param string) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// 使用 channel 来传递错误
			errCh := make(chan error, 1)
			go func() {
				errCh <- next(ctx, param)
			}()

			select {
			case err := <-errCh:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// RetryMiddleware 重试中间件
// 在任务失败时自动重试（注意：XXL-JOB 本身也支持重试，此中间件用于客户端重试）
func RetryMiddleware(maxRetries int, backoff time.Duration) Middleware {
	return func(next TaskHandler) TaskHandler {
		return func(ctx context.Context, param string) error {
			var lastErr error
			for i := 0; i <= maxRetries; i++ {
				if i > 0 {
					// 等待后重试
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(backoff):
					}
					backoff *= 2 // 指数退避
				}

				err := next(ctx, param)
				if err == nil {
					return nil
				}
				lastErr = err

				// 检查上下文是否已取消
				if ctx.Err() != nil {
					return ctx.Err()
				}
			}
			return lastErr
		}
	}
}
