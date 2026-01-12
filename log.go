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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-anyway/framework-log"

	xxl "github.com/xxl-job/xxl-job-executor-go"
	"go.uber.org/zap"
)

const (
	// defaultLogPageSize 默认日志分页大小（行数）
	defaultLogPageSize = 1000
	// maxLogFileSize 最大日志文件大小（10MB），超过此大小强制分页
	maxLogFileSize = 10 * 1024 * 1024
)

// contextKey 用于在 context 中存储 LogWriter 的 key
type contextKey string

const logWriterKey = contextKey("xxljob_log_writer")

// LogWriterFromContext 从 context 中获取 LogWriter
// 任务执行过程中可以使用此函数获取日志写入器
func LogWriterFromContext(ctx context.Context) LogWriter {
	if ctx == nil {
		return nil
	}
	if writer, ok := ctx.Value(logWriterKey).(LogWriter); ok {
		return writer
	}
	return nil
}

// logWriter 日志写入器实现
type logWriter struct {
	logPath string
	logID   int64
	file    *os.File
	mu      sync.Mutex
}

// newLogWriter 创建新的日志写入器
func newLogWriter(logPath string, logID int64) (*logWriter, error) {
	if logPath == "" || logID == 0 {
		return nil, fmt.Errorf("log path or log ID is empty")
	}

	// 构建日志文件路径
	logFileName := fmt.Sprintf("jobhandler-%d.log", logID)
	logFilePath := filepath.Join(logPath, logFileName)

	// 打开或创建日志文件（追加模式）
	// #nosec G302,G304 -- 日志文件需要可读权限，文件路径来自配置
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &logWriter{
		logPath: logPath,
		logID:   logID,
		file:    file,
	}, nil
}

// Write 写入一行日志（自动添加时间戳）
func (w *logWriter) Write(format string, args ...interface{}) {
	if w == nil || w.file == nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	content := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), content)
	if _, err := w.file.WriteString(logLine); err != nil {
		// 写入失败时记录警告，但不影响任务执行
		// 这里不能使用 log 包，因为可能导致循环依赖
		_ = err
		return
	}

	// 立即同步到磁盘，确保调度中心能及时拉取到日志
	if err := w.file.Sync(); err != nil {
		// 同步失败不影响任务执行，但可能导致日志延迟
		_ = err
	}
}

// WriteLine 写入一行日志（不添加时间戳）
func (w *logWriter) WriteLine(line string) {
	if w == nil || w.file == nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	if _, err := w.file.WriteString(line); err != nil {
		_ = err
		return
	}

	// 立即同步到磁盘，确保调度中心能及时拉取到日志
	if err := w.file.Sync(); err != nil {
		// 同步失败不影响任务执行，但可能导致日志延迟
		_ = err
	}
}

// Close 关闭日志写入器
func (w *logWriter) Close() error {
	if w == nil || w.file == nil {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 关闭前先同步，确保所有日志都已写入磁盘
	if err := w.file.Sync(); err != nil {
		// 同步失败不影响关闭
		_ = err
	}

	err := w.file.Close()
	w.file = nil
	return err
}

// handleLogRequest 处理日志查询请求（管理端查询日志时调用）
// 优化：支持真正的分页读取，避免大文件内存占用
func handleLogRequest(req *xxl.LogReq, logPath string) *xxl.LogRes {
	if req == nil {
		log.Error("XXL-JOB log request is nil")
		return &xxl.LogRes{
			Code: 500,
			Msg:  "log request is nil",
		}
	}

	if logPath == "" {
		log.Error("XXL-JOB log path not configured")
		return &xxl.LogRes{
			Code: 500,
			Msg:  "log path not configured",
		}
	}

	// 构建日志文件路径
	logFileName := fmt.Sprintf("jobhandler-%d.log", req.LogID)
	logFilePath := filepath.Join(logPath, logFileName)

	// 检查文件是否存在
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		log.Warn("XXL-JOB log file not found",
			zap.String("log_file", logFilePath),
			zap.Int64("log_id", req.LogID),
		)
		return &xxl.LogRes{
			Code: 500,
			Msg:  fmt.Sprintf("log file not found: %s", logFilePath),
		}
	}

	// 读取日志文件内容（优化：按行分页读取）
	result, err := readLogFileWithPagination(logFilePath, req.FromLineNum, defaultLogPageSize)
	if err != nil {
		log.Warn("XXL-JOB failed to read log file",
			zap.String("log_file", logFilePath),
			zap.Int64("log_id", req.LogID),
			zap.Int("from_line", req.FromLineNum),
			zap.Error(err),
		)
		return &xxl.LogRes{
			Code: 500,
			Msg:  fmt.Sprintf("failed to read log file: %v", err),
		}
	}

	// 返回日志内容
	return &xxl.LogRes{
		Code: 200,
		Msg:  "success",
		Content: xxl.LogResContent{
			FromLineNum: req.FromLineNum,
			ToLineNum:   result.ToLineNum,
			LogContent:  result.Content,
			IsEnd:       result.IsEnd,
		},
	}
}

// logReadResult 日志读取结果
type logReadResult struct {
	Content   string // 日志内容
	ToLineNum int    // 结束行号
	IsEnd     bool   // 是否已读取到文件末尾
}

// readLogFileWithPagination 按行分页读取日志文件内容（优化版本）
// 使用 bufio.Scanner 逐行读取，避免大文件内存占用
// 注意：XXL-JOB 的行号约定从 0 开始（第一行是 0，第二行是 1，以此类推）
// FromLineNum = 0 表示从第 1 行开始读取
func readLogFileWithPagination(filePath string, fromLineNum int, pageSize int) (*logReadResult, error) {
	// 检查文件是否存在
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("log file not found: %s", filePath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat log file: %w", err)
	}

	// 检查文件大小，如果文件为空，直接返回
	if fileInfo.Size() == 0 {
		// 空文件，返回 fromLineNum（如果 < 0 则返回 0）
		toLineNum := fromLineNum
		if toLineNum < 0 {
			toLineNum = 0
		}
		return &logReadResult{
			Content:   "",
			ToLineNum: toLineNum,
			IsEnd:     true,
		}, nil
	}

	// 如果文件很大，强制使用分页（防止内存占用过大）
	forcePagination := fileInfo.Size() > maxLogFileSize
	if forcePagination && pageSize <= 0 {
		pageSize = defaultLogPageSize
	}
	if pageSize <= 0 {
		pageSize = defaultLogPageSize
	}

	// XXL-JOB 行号约定：从 0 开始
	// FromLineNum = 0 表示从第 1 行开始（文件中的第 1 行对应行号 0）
	// 如果 fromLineNum < 0，则从第 1 行开始
	startLineIndex := fromLineNum
	if startLineIndex < 0 {
		startLineIndex = 0
	}

	// 打开文件
	// #nosec G304 -- 文件路径来自配置，已验证
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// 使用 bufio.Scanner 逐行读取（内存效率高）
	scanner := bufio.NewScanner(file)
	// 设置缓冲区大小（默认 64KB，对于超长行可以增大）
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 最大支持 1MB 的单行

	var lines []string
	lineIndex := -1 // 文件中的行索引（从 0 开始，对应 XXL-JOB 的行号）
	toLineNum := startLineIndex

	// 逐行读取
	for scanner.Scan() {
		lineIndex++ // 行号从 0 开始

		// 跳过 startLineIndex 之前的行
		if lineIndex < startLineIndex {
			continue
		}

		// 读取指定数量的行
		line := scanner.Text()
		lines = append(lines, line)
		toLineNum = lineIndex // XXL-JOB 约定：ToLineNum 是已读取的最后一行（从 0 开始）

		// 如果达到分页大小，停止读取
		if len(lines) >= pageSize {
			break
		}
	}

	// 检查扫描错误
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan log file: %w", err)
	}

	// 判断是否已读取到文件末尾
	// 如果读取的行数少于分页大小，说明已经到文件末尾
	isEnd := len(lines) < pageSize

	// 如果 fromLineNum 超出文件行数，返回空内容
	if startLineIndex > lineIndex {
		// 返回的 ToLineNum 应该是文件的最后一行索引（从 0 开始）
		toLineNum = lineIndex
		if toLineNum < 0 {
			toLineNum = 0
		}
		return &logReadResult{
			Content:   "",
			ToLineNum: toLineNum,
			IsEnd:     true,
		}, nil
	}

	// 拼接日志内容
	// XXL-JOB 要求：每行末尾必须有换行符（包括最后一行）
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}

	return &logReadResult{
		Content:   content,
		ToLineNum: toLineNum,
		IsEnd:     isEnd,
	}, nil
}

// cleanupOldLogs 清理旧日志文件（后台任务）
// 优化：按文件修改时间清理，支持任务执行过程中的日志追加
func cleanupOldLogs(logPath string, retentionDays int) {
	if logPath == "" || retentionDays <= 0 {
		return
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	entries, err := os.ReadDir(logPath)
	if err != nil {
		log.Warn("Failed to read log directory",
			zap.String("log_path", logPath),
			zap.Error(err),
		)
		return
	}

	var deletedCount int
	var totalSize int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 只处理 jobhandler-*.log 文件
		if !strings.HasPrefix(entry.Name(), "jobhandler-") || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 删除超过保留天数的日志文件（基于修改时间）
		// 注意：使用 ModTime 而不是创建时间，因为任务执行过程中会追加日志
		if info.ModTime().Before(cutoffTime) {
			filePath := filepath.Join(logPath, entry.Name())
			if err := os.Remove(filePath); err != nil {
				log.Warn("Failed to remove old log file",
					zap.String("log_file", filePath),
					zap.Error(err),
				)
			} else {
				deletedCount++
				totalSize += info.Size()
			}
		}
	}

	// 汇总清理结果
	if deletedCount > 0 {
		log.Info("Cleaned up old XXL-JOB log files",
			zap.String("log_path", logPath),
			zap.Int("deleted_count", deletedCount),
			zap.Int64("freed_size_mb", totalSize/(1024*1024)),
			zap.Int("retention_days", retentionDays),
		)
	}
}
