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

// NewExecutorBuilder 创建执行器构建器
// 示例：
//
//	executor, err := NewExecutorBuilder().
//	    ServerAddr("http://localhost:8080/xxl-job-admin").
//	    RegistryKey("my-executor").
//	    ExecutorPort("9999").
//	    LogPath("./logs/xxl-job").
//	    Trace(true).
//	    Build()
func NewExecutorBuilder() *ExecutorBuilder {
	return &ExecutorBuilder{
		builder: NewOptionsBuilder(),
	}
}

// ExecutorBuilder 执行器构建器
type ExecutorBuilder struct {
	builder *OptionsBuilder
}

// ServerAddr 设置调度中心地址
func (b *ExecutorBuilder) ServerAddr(addr string) *ExecutorBuilder {
	b.builder.ServerAddr(addr)
	return b
}

// AccessToken 设置访问令牌
func (b *ExecutorBuilder) AccessToken(token string) *ExecutorBuilder {
	b.builder.AccessToken(token)
	return b
}

// ExecutorIP 设置执行器 IP
func (b *ExecutorBuilder) ExecutorIP(ip string) *ExecutorBuilder {
	b.builder.ExecutorIP(ip)
	return b
}

// ExecutorPort 设置执行器端口
func (b *ExecutorBuilder) ExecutorPort(port string) *ExecutorBuilder {
	b.builder.ExecutorPort(port)
	return b
}

// RegistryKey 设置执行器注册名称
func (b *ExecutorBuilder) RegistryKey(key string) *ExecutorBuilder {
	b.builder.RegistryKey(key)
	return b
}

// LogPath 设置日志文件路径
func (b *ExecutorBuilder) LogPath(path string) *ExecutorBuilder {
	b.builder.LogPath(path)
	return b
}

// LogRetentionDays 设置日志保留天数
func (b *ExecutorBuilder) LogRetentionDays(days int) *ExecutorBuilder {
	b.builder.LogRetentionDays(days)
	return b
}

// Trace 启用/禁用追踪
func (b *ExecutorBuilder) Trace(enabled bool) *ExecutorBuilder {
	b.builder.Trace(enabled)
	return b
}

// Middleware 添加中间件
func (b *ExecutorBuilder) Middleware(middleware Middleware) *ExecutorBuilder {
	b.builder.Middleware(middleware)
	return b
}

// Middlewares 批量添加中间件
func (b *ExecutorBuilder) Middlewares(middlewares ...Middleware) *ExecutorBuilder {
	b.builder.Middlewares(middlewares...)
	return b
}

// QuietMode 启用/禁用静默模式（不输出心跳/注册日志）
func (b *ExecutorBuilder) QuietMode(enabled bool) *ExecutorBuilder {
	b.builder.QuietMode(enabled)
	return b
}

// Build 构建执行器
func (b *ExecutorBuilder) Build() (Executor, error) {
	opts, err := b.builder.Build()
	if err != nil {
		return nil, err
	}
	return NewExecutorWithOptions(opts)
}
