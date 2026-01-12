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
	"fmt"
)

// Config XXL-JOB 配置结构体（用于从配置文件创建）
type Config struct {
	Enabled          bool   `yaml:"enabled" env:"XXL_JOB_ENABLED" default:"false"`
	ServerAddr       string `yaml:"server_addr" env:"XXL_JOB_SERVER_ADDR" required:"true"`
	AccessToken      string `yaml:"access_token" env:"XXL_JOB_ACCESS_TOKEN"`
	ExecutorIP       string `yaml:"executor_ip" env:"XXL_JOB_EXECUTOR_IP"`
	ExecutorPort     string `yaml:"executor_port" env:"XXL_JOB_EXECUTOR_PORT" default:"9999"`
	RegistryKey      string `yaml:"registry_key" env:"XXL_JOB_REGISTRY_KEY" required:"true"`
	LogPath          string `yaml:"log_path" env:"XXL_JOB_LOG_PATH" default:"./logs/xxl-job"`
	LogRetentionDays int    `yaml:"log_retention_days" env:"XXL_JOB_LOG_RETENTION_DAYS" default:"30"`
	EnableTrace      bool   `yaml:"enable_trace" env:"XXL_JOB_ENABLE_TRACE" default:"true"`
	QuietMode        bool   `yaml:"quiet_mode" env:"XXL_JOB_QUIET_MODE" default:"false"`
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("xxl-job config cannot be nil")
	}
	if !c.Enabled {
		return nil // 如果未启用，不需要验证
	}
	if c.ServerAddr == "" {
		return fmt.Errorf("xxl-job server_addr is required")
	}
	if c.RegistryKey == "" {
		return fmt.Errorf("xxl-job registry_key is required")
	}
	if c.ExecutorPort == "" {
		return fmt.Errorf("xxl-job executor_port is required")
	}
	return nil
}

// ToOptions 转换为 executorOptions
func (c *Config) ToOptions() (*executorOptions, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if !c.Enabled {
		return nil, fmt.Errorf("xxl-job is not enabled")
	}

	opts := NewOptions()
	opts.serverAddr = c.ServerAddr
	opts.accessToken = c.AccessToken
	opts.executorIP = c.ExecutorIP
	opts.executorPort = c.ExecutorPort
	opts.registryKey = c.RegistryKey
	opts.logPath = c.LogPath
	opts.logRetentionDays = c.LogRetentionDays
	opts.enableTrace = c.EnableTrace
	opts.quietMode = c.QuietMode

	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	return opts, nil
}

// executorOptions XXL-JOB 执行器选项（内部使用）
// 使用 Builder 模式构建
type executorOptions struct {
	serverAddr       string
	accessToken      string
	executorIP       string
	executorPort     string
	registryKey      string
	logPath          string
	logRetentionDays int
	enableTrace      bool
	quietMode        bool // 静默模式：不输出心跳/注册日志
	middlewares      []Middleware
}

// Option 配置选项函数类型
type Option func(*executorOptions)

// NewOptions 创建新的选项（使用默认值）
func NewOptions() *executorOptions {
	return &executorOptions{
		executorPort:     "9999",
		logRetentionDays: 30,
		enableTrace:      false,
		quietMode:        false, // 默认输出心跳日志
		middlewares:      make([]Middleware, 0),
	}
}

// WithServerAddr 设置调度中心地址
func WithServerAddr(addr string) Option {
	return func(o *executorOptions) {
		o.serverAddr = addr
	}
}

// WithAccessToken 设置访问令牌
func WithAccessToken(token string) Option {
	return func(o *executorOptions) {
		o.accessToken = token
	}
}

// WithExecutorIP 设置执行器 IP
func WithExecutorIP(ip string) Option {
	return func(o *executorOptions) {
		o.executorIP = ip
	}
}

// WithExecutorPort 设置执行器端口
func WithExecutorPort(port string) Option {
	return func(o *executorOptions) {
		o.executorPort = port
	}
}

// WithRegistryKey 设置执行器注册名称（AppName）
func WithRegistryKey(key string) Option {
	return func(o *executorOptions) {
		o.registryKey = key
	}
}

// WithLogPath 设置日志文件路径
func WithLogPath(path string) Option {
	return func(o *executorOptions) {
		o.logPath = path
	}
}

// WithLogRetentionDays 设置日志保留天数
func WithLogRetentionDays(days int) Option {
	return func(o *executorOptions) {
		o.logRetentionDays = days
	}
}

// WithTrace 启用/禁用追踪
func WithTrace(enabled bool) Option {
	return func(o *executorOptions) {
		o.enableTrace = enabled
	}
}

// WithQuietMode 启用/禁用静默模式（不输出心跳/注册日志）
func WithQuietMode(enabled bool) Option {
	return func(o *executorOptions) {
		o.quietMode = enabled
	}
}

// WithMiddleware 添加中间件
func WithMiddleware(middleware Middleware) Option {
	return func(o *executorOptions) {
		o.middlewares = append(o.middlewares, middleware)
	}
}

// WithMiddlewares 批量添加中间件
func WithMiddlewares(middlewares ...Middleware) Option {
	return func(o *executorOptions) {
		o.middlewares = append(o.middlewares, middlewares...)
	}
}

// Validate 验证选项
func (o *executorOptions) Validate() error {
	if o.serverAddr == "" {
		return fmt.Errorf("server address is required")
	}
	if o.registryKey == "" {
		return fmt.Errorf("registry key is required")
	}
	if o.executorPort == "" {
		return fmt.Errorf("executor port is required")
	}
	return nil
}

// NewFromConfig 从配置创建执行器
func NewFromConfig(cfg *Config) (Executor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("xxl-job is not enabled")
	}

	builder := NewExecutorBuilder().
		ServerAddr(cfg.ServerAddr).
		RegistryKey(cfg.RegistryKey).
		ExecutorPort(cfg.ExecutorPort).
		LogPath(cfg.LogPath).
		LogRetentionDays(cfg.LogRetentionDays).
		Trace(cfg.EnableTrace).
		QuietMode(cfg.QuietMode)

	if cfg.AccessToken != "" {
		builder = builder.AccessToken(cfg.AccessToken)
	}
	if cfg.ExecutorIP != "" {
		builder = builder.ExecutorIP(cfg.ExecutorIP)
	}

	return builder.Build()
}

// OptionsBuilder 选项构建器（链式调用）
type OptionsBuilder struct {
	opts *executorOptions
}

// NewOptionsBuilder 创建新的选项构建器
func NewOptionsBuilder() *OptionsBuilder {
	return &OptionsBuilder{
		opts: NewOptions(),
	}
}

// ServerAddr 设置调度中心地址
func (b *OptionsBuilder) ServerAddr(addr string) *OptionsBuilder {
	b.opts.serverAddr = addr
	return b
}

// AccessToken 设置访问令牌
func (b *OptionsBuilder) AccessToken(token string) *OptionsBuilder {
	b.opts.accessToken = token
	return b
}

// ExecutorIP 设置执行器 IP
func (b *OptionsBuilder) ExecutorIP(ip string) *OptionsBuilder {
	b.opts.executorIP = ip
	return b
}

// ExecutorPort 设置执行器端口
func (b *OptionsBuilder) ExecutorPort(port string) *OptionsBuilder {
	b.opts.executorPort = port
	return b
}

// RegistryKey 设置执行器注册名称
func (b *OptionsBuilder) RegistryKey(key string) *OptionsBuilder {
	b.opts.registryKey = key
	return b
}

// LogPath 设置日志文件路径
func (b *OptionsBuilder) LogPath(path string) *OptionsBuilder {
	b.opts.logPath = path
	return b
}

// LogRetentionDays 设置日志保留天数
func (b *OptionsBuilder) LogRetentionDays(days int) *OptionsBuilder {
	b.opts.logRetentionDays = days
	return b
}

// Trace 启用/禁用追踪
func (b *OptionsBuilder) Trace(enabled bool) *OptionsBuilder {
	b.opts.enableTrace = enabled
	return b
}

// QuietMode 启用/禁用静默模式（不输出心跳/注册日志）
func (b *OptionsBuilder) QuietMode(enabled bool) *OptionsBuilder {
	b.opts.quietMode = enabled
	return b
}

// Middleware 添加中间件
func (b *OptionsBuilder) Middleware(middleware Middleware) *OptionsBuilder {
	b.opts.middlewares = append(b.opts.middlewares, middleware)
	return b
}

// Middlewares 批量添加中间件
func (b *OptionsBuilder) Middlewares(middlewares ...Middleware) *OptionsBuilder {
	b.opts.middlewares = append(b.opts.middlewares, middlewares...)
	return b
}

// Build 构建选项
func (b *OptionsBuilder) Build() (*executorOptions, error) {
	if err := b.opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}
	return b.opts, nil
}

// WithServerAddr 辅助方法：链式设置选项
func (o *executorOptions) WithServerAddr(addr string) *executorOptions {
	o.serverAddr = addr
	return o
}

func (o *executorOptions) WithAccessToken(token string) *executorOptions {
	o.accessToken = token
	return o
}

func (o *executorOptions) WithExecutorIP(ip string) *executorOptions {
	o.executorIP = ip
	return o
}

func (o *executorOptions) WithExecutorPort(port string) *executorOptions {
	o.executorPort = port
	return o
}

func (o *executorOptions) WithRegistryKey(key string) *executorOptions {
	o.registryKey = key
	return o
}

func (o *executorOptions) WithLogPath(path string) *executorOptions {
	o.logPath = path
	return o
}

func (o *executorOptions) WithLogRetentionDays(days int) *executorOptions {
	o.logRetentionDays = days
	return o
}

func (o *executorOptions) WithTrace(enabled bool) *executorOptions {
	o.enableTrace = enabled
	return o
}

func (o *executorOptions) WithQuietMode(enabled bool) *executorOptions {
	o.quietMode = enabled
	return o
}

func (o *executorOptions) WithMiddleware(middleware Middleware) *executorOptions {
	o.middlewares = append(o.middlewares, middleware)
	return o
}

func (o *executorOptions) WithMiddlewares(middlewares ...Middleware) *executorOptions {
	o.middlewares = append(o.middlewares, middlewares...)
	return o
}
