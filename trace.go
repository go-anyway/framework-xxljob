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

	"github.com/go-anyway/framework-log"
	"github.com/go-anyway/framework-metrics"
	pkgtrace "github.com/go-anyway/framework-trace"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// executeTaskWithTrace 带追踪的任务执行包装器
// 统一处理日志、追踪、Metrics 等横切关注点
func executeTaskWithTrace(
	ctx context.Context,
	taskName string,
	param string,
	logID int64,
	handler TaskHandler,
	enableTrace bool,
) (result string, err error) {
	startTime := time.Now()

	// 创建追踪 span
	var span trace.Span
	if enableTrace {
		ctx, span = pkgtrace.StartSpan(ctx, "xxljob.task.execute",
			trace.WithAttributes(
				attribute.String("xxljob.task.name", taskName),
				attribute.String("xxljob.task.param", param),
				attribute.Int64("xxljob.log.id", logID),
			),
		)
		defer span.End()
	}

	// 记录任务开始日志（同时写入文件日志，如果 LogWriter 存在）
	logWriter := LogWriterFromContext(ctx)
	if logWriter != nil {
		logWriter.Write("XXL-JOB task [%s] started, param: %s", taskName, param)
	}

	log.FromContext(ctx).Info("XXL-JOB task started",
		zap.String("task_name", taskName),
		zap.String("param", param),
		zap.Int64("log_id", logID),
	)

	// 执行任务
	err = handler(ctx, param)
	duration := time.Since(startTime)

	// 记录 Metrics
	if metrics.IsEnabled() {
		status := "success"
		if err != nil {
			status = "error"
		}
		metrics.XXLJobTaskTotal.WithLabelValues(taskName, status).Inc()
		metrics.XXLJobTaskDuration.WithLabelValues(taskName).Observe(duration.Seconds())
	}

	// 处理结果
	if err != nil {
		// 记录错误日志（同时写入文件日志）
		if logWriter != nil {
			logWriter.Write("XXL-JOB task [%s] failed after %v: %v", taskName, duration, err)
		}

		log.FromContext(ctx).Error("XXL-JOB task failed",
			zap.String("task_name", taskName),
			zap.String("param", param),
			zap.Int64("log_id", logID),
			zap.Duration("duration", duration),
			zap.Error(err),
		)

		// 更新追踪状态
		if enableTrace && span != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
			span.SetAttributes(
				attribute.String("xxljob.task.status", "failed"),
				attribute.String("xxljob.task.error", err.Error()),
			)
		}

		result = fmt.Sprintf("FAIL: %v", err)
	} else {
		// 记录成功日志（同时写入文件日志）
		if logWriter != nil {
			logWriter.Write("XXL-JOB task [%s] completed successfully in %v", taskName, duration)
		}

		log.FromContext(ctx).Info("XXL-JOB task completed",
			zap.String("task_name", taskName),
			zap.String("param", param),
			zap.Int64("log_id", logID),
			zap.Duration("duration", duration),
		)

		// 更新追踪状态
		if enableTrace && span != nil {
			span.SetStatus(codes.Ok, "")
			span.SetAttributes(
				attribute.String("xxljob.task.status", "success"),
				attribute.Float64("xxljob.task.duration_ms", float64(duration.Milliseconds())),
			)
		}

		result = "SUCCESS"
	}

	return result, nil
}
