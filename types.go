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
	"time"
)

// Executor XXL-JOB 执行器接口
// 提供任务注册、启动、停止等核心功能
type Executor interface {
	// RegTask 注册任务
	// taskName: 任务名称，必须与 XXL-JOB 管理端配置的 JobHandler 一致
	// handler: 任务处理函数
	RegTask(taskName string, handler TaskHandler) error

	// Run 启动执行器（阻塞调用）
	// 通常在单独的 goroutine 中调用
	Run() error

	// Stop 停止执行器
	Stop() error

	// IsRunning 检查执行器是否正在运行
	IsRunning() bool

	// GetTaskNames 获取所有已注册的任务名称
	GetTaskNames() []string
}

// TaskHandler 任务处理器函数类型
// ctx: 任务执行上下文，包含取消信号和日志写入器
// param: 任务参数（字符串格式，通常为 JSON）
// 返回: 错误信息，nil 表示成功
type TaskHandler func(ctx context.Context, param string) error

// TaskInfo 任务信息
type TaskInfo struct {
	Name         string      // 任务名称
	Handler      TaskHandler // 任务处理器
	RegisteredAt time.Time   // 注册时间
}

// Middleware 中间件函数类型
// 用于在任务执行前后添加额外逻辑（如日志、追踪、Metrics 等）
type Middleware func(next TaskHandler) TaskHandler

// LogWriter 日志写入器接口
// 任务执行过程中可以使用此接口写入多行日志
type LogWriter interface {
	// Write 写入一行日志（自动添加时间戳）
	Write(format string, args ...interface{})

	// WriteLine 写入一行日志（不添加时间戳）
	WriteLine(line string)
}

// HealthStatus 健康状态
type HealthStatus struct {
	Running   bool      // 是否正在运行
	TaskCount int       // 已注册任务数量
	StartedAt time.Time // 启动时间
	LastError error     // 最后一次错误
}
