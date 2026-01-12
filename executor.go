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
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-anyway/framework-log"

	xxl "github.com/xxl-job/xxl-job-executor-go"
	"go.uber.org/zap"
)

// executorImpl 执行器实现
type executorImpl struct {
	executor    xxl.Executor
	opts        *executorOptions
	registry    *TaskRegistry
	running     bool
	runningMu   sync.RWMutex
	startedAt   time.Time
	lastError   error
	lastErrorMu sync.RWMutex
}

// NewExecutorWithOptions 使用选项创建新的执行器
func NewExecutorWithOptions(opts *executorOptions) (Executor, error) {
	if opts == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}

	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// 构建 XXL-JOB SDK 选项
	xxlOpts := []xxl.Option{
		xxl.ServerAddr(opts.serverAddr),
		xxl.RegistryKey(opts.registryKey),
		xxl.ExecutorPort(opts.executorPort),
	}

	// 可选配置
	if opts.accessToken != "" {
		xxlOpts = append(xxlOpts, xxl.AccessToken(opts.accessToken))
	}

	if opts.executorIP != "" {
		xxlOpts = append(xxlOpts, xxl.ExecutorIp(opts.executorIP))
	}

	// 创建真实的执行器
	xxlExecutor := xxl.NewExecutor(xxlOpts...)

	// 配置日志处理器（如果指定了日志路径）
	// 注意：必须在 Init 之前设置，确保日志处理器被正确注册
	if opts.logPath != "" {
		// 确保日志目录存在
		// #nosec G301 -- 日志目录需要可读权限
		if err := os.MkdirAll(opts.logPath, 0755); err != nil {
			log.Warn("Failed to create log directory",
				zap.String("log_path", opts.logPath),
				zap.Error(err),
			)
		} else {
			// 设置自定义日志处理器（用于管理端查询日志）
			// 注意：必须在 Init 之前注册，否则可能被 SDK 的默认处理器覆盖
			xxlExecutor.LogHandler(func(req *xxl.LogReq) *xxl.LogRes {
				return handleLogRequest(req, opts.logPath)
			})

			// 启动后台任务清理旧日志
			if opts.logRetentionDays > 0 {
				go func() {
					ticker := time.NewTicker(1 * time.Hour) // 每小时清理一次
					defer ticker.Stop()
					for range ticker.C {
						cleanupOldLogs(opts.logPath, opts.logRetentionDays)
					}
				}()
			}
		}
	}

	// 初始化执行器（必须调用，否则 taskList 为 nil 会导致 panic）
	xxlExecutor.Init(xxlOpts...)

	// 如果启用了静默模式，设置日志拦截器
	if opts.quietMode {
		setupLogInterceptor(true)
	}

	return &executorImpl{
		executor: xxlExecutor,
		opts:     opts,
		registry: NewTaskRegistry(),
		running:  false,
	}, nil
}

// RegTask 注册任务
func (e *executorImpl) RegTask(taskName string, handler TaskHandler) error {
	e.runningMu.RLock()
	if e.running {
		e.runningMu.RUnlock()
		return fmt.Errorf("cannot register task after executor started")
	}
	e.runningMu.RUnlock()

	// 应用中间件链
	wrappedHandler := applyMiddlewares(handler, e.opts.middlewares)

	// 注册到任务注册表
	if err := e.registry.Register(taskName, wrappedHandler); err != nil {
		return fmt.Errorf("failed to register task: %w", err)
	}

	// 注册到真实执行器
	// SDK 的 TaskFunc 返回 string，我们需要将 error 转换为 string
	e.executor.RegTask(taskName, func(ctx context.Context, param *xxl.RunReq) string {
		// 提取参数
		paramStr := ""
		logID := int64(0)
		if param != nil {
			if param.ExecutorParams != "" {
				paramStr = param.ExecutorParams
			}
			logID = param.LogID
		}

		// 如果配置了日志路径，创建日志写入器并注入到 context
		if e.opts.logPath != "" && logID > 0 {
			logWriter, logErr := newLogWriter(e.opts.logPath, logID)
			if logErr == nil {
				// 将日志写入器注入到 context
				ctx = context.WithValue(ctx, logWriterKey, logWriter)
				// 确保任务执行完成后关闭日志文件
				defer func() {
					if closeErr := logWriter.Close(); closeErr != nil {
						log.Warn("Failed to close log writer",
							zap.Int64("log_id", logID),
							zap.Error(closeErr),
						)
					}
				}()
			} else {
				// 日志写入器创建失败，记录警告但不影响任务执行
				log.Warn("Failed to create log writer",
					zap.Int64("log_id", logID),
					zap.Error(logErr),
				)
			}
		}

		// 使用追踪包装器执行任务（统一日志收集、追踪、Metrics）
		result, err := executeTaskWithTrace(
			ctx,
			taskName,
			paramStr,
			logID,
			wrappedHandler,
			e.opts.enableTrace,
		)

		// 记录错误（用于健康检查）
		if err != nil {
			e.lastErrorMu.Lock()
			e.lastError = err
			e.lastErrorMu.Unlock()
		}

		return result
	})

	return nil
}

// Run 启动执行器
func (e *executorImpl) Run() error {
	e.runningMu.Lock()
	if e.running {
		e.runningMu.Unlock()
		return fmt.Errorf("executor already running")
	}
	e.running = true
	e.startedAt = time.Now()
	e.runningMu.Unlock()

	// 输出启动信息
	log.Info("XXL-JOB executor registered and started",
		zap.String("server_addr", e.opts.serverAddr),
		zap.String("registry_key", e.opts.registryKey),
		zap.String("executor_port", e.opts.executorPort),
		zap.String("executor_ip", e.opts.executorIP),
		zap.Int("task_count", e.registry.Count()),
		zap.Bool("trace_enabled", e.opts.enableTrace),
	)

	// 输出已注册的任务列表
	taskNames := e.registry.GetNames()
	if len(taskNames) > 0 {
		log.Info("XXL-JOB tasks registered",
			zap.Strings("task_names", taskNames),
			zap.String("registry_key", e.opts.registryKey),
		)
	}

	// 启动真实执行器（会阻塞）
	return e.executor.Run()
}

// Stop 停止执行器
func (e *executorImpl) Stop() error {
	e.runningMu.Lock()
	defer e.runningMu.Unlock()

	if !e.running {
		return nil
	}

	log.Info("Stopping XXL-JOB executor")
	e.running = false

	// 调用 SDK 的 Stop 方法
	e.executor.Stop()
	return nil
}

// IsRunning 检查执行器是否正在运行
func (e *executorImpl) IsRunning() bool {
	e.runningMu.RLock()
	defer e.runningMu.RUnlock()
	return e.running
}

// GetTaskNames 获取所有已注册的任务名称
func (e *executorImpl) GetTaskNames() []string {
	return e.registry.GetNames()
}

// GetHealthStatus 获取健康状态（内部方法）
func (e *executorImpl) GetHealthStatus() *HealthStatus {
	e.runningMu.RLock()
	running := e.running
	startedAt := e.startedAt
	e.runningMu.RUnlock()

	e.lastErrorMu.RLock()
	lastError := e.lastError
	e.lastErrorMu.RUnlock()

	return &HealthStatus{
		Running:   running,
		TaskCount: e.registry.Count(),
		StartedAt: startedAt,
		LastError: lastError,
	}
}

// logInterceptor 日志拦截器，用于拦截和过滤 SDK 的输出
type logInterceptor struct {
	originalStdout *os.File
	quietMode      bool
	mu             sync.Mutex
}

var (
	logInterceptorOnce sync.Once
	logInterceptorInst *logInterceptor
)

// setupLogInterceptor 设置日志拦截器（仅设置一次）
// 注意：拦截标准输出会影响全局，但这是统一日志输出的必要方案
// 在静默模式下，会过滤掉心跳/注册成功的日志
func setupLogInterceptor(quietMode bool) {
	logInterceptorOnce.Do(func() {
		// 保存原始标准输出
		originalStdout := os.Stdout

		// 创建管道
		r, w, err := os.Pipe()
		if err != nil {
			log.Warn("Failed to create pipe for log interceptor", zap.Error(err))
			return
		}

		// 替换标准输出
		os.Stdout = w

		interceptor := &logInterceptor{
			originalStdout: originalStdout,
			quietMode:      quietMode,
		}
		logInterceptorInst = interceptor

		// 启动 goroutine 读取并过滤日志
		go interceptor.interceptLogs(r, originalStdout)
	})
}

// interceptLogs 拦截并过滤日志
func (li *logInterceptor) interceptLogs(r *os.File, originalStdout *os.File) {
	defer func() {
		// 确保在退出时恢复原始输出
		if r := recover(); r != nil {
			log.Error("Log interceptor panic", zap.Any("panic", r))
			os.Stdout = originalStdout
		}
	}()

	scanner := bufio.NewScanner(r)
	// 设置缓冲区大小，支持较长的日志行
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 最大支持 1MB 的单行

	for scanner.Scan() {
		line := scanner.Text()

		// 在静默模式下，过滤掉心跳/注册成功的日志
		if li.quietMode {
			if shouldFilterHeartbeatLog(line) {
				// 静默模式下，不输出心跳日志
				continue
			}
		}

		// 统一使用项目的日志系统输出
		// 判断日志级别（简单判断，可以根据实际需要调整）
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, "error") || strings.Contains(lineLower, "fatal") {
			log.Error("XXL-JOB SDK", zap.String("message", line))
		} else if strings.Contains(lineLower, "warn") || strings.Contains(lineLower, "warning") {
			log.Warn("XXL-JOB SDK", zap.String("message", line))
		} else {
			// 非静默模式下，输出所有日志；静默模式下，只输出非心跳日志
			log.Info("XXL-JOB SDK", zap.String("message", line))
		}
	}

	// 如果扫描出错，恢复原始输出
	if err := scanner.Err(); err != nil {
		log.Warn("Log interceptor scanner error", zap.Error(err))
		os.Stdout = originalStdout
	}
}

// shouldFilterHeartbeatLog 判断是否应该过滤心跳日志
func shouldFilterHeartbeatLog(line string) bool {
	// 过滤包含 "执行器注册成功" 的日志
	if strings.Contains(line, "执行器注册成功") {
		return true
	}

	// 过滤包含 "code":200 且 "msg":null 的日志（心跳成功响应）
	if strings.Contains(line, `"code":200`) && strings.Contains(line, `"msg":null`) {
		return true
	}

	// 可以根据需要添加其他过滤规则
	return false
}
