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
	"sync"
	"time"
)

// TaskRegistry 任务注册表
// 统一管理所有已注册的任务
type TaskRegistry struct {
	mu    sync.RWMutex
	tasks map[string]*TaskInfo
}

// NewTaskRegistry 创建新的任务注册表
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{
		tasks: make(map[string]*TaskInfo),
	}
}

// Register 注册任务
func (r *TaskRegistry) Register(name string, handler TaskHandler) error {
	if name == "" {
		return fmt.Errorf("task name cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("task handler cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[name]; exists {
		return fmt.Errorf("task %s already registered", name)
	}

	r.tasks[name] = &TaskInfo{
		Name:         name,
		Handler:      handler,
		RegisteredAt: time.Now(),
	}

	return nil
}

// Get 获取任务信息
func (r *TaskRegistry) Get(name string) (*TaskInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, exists := r.tasks[name]
	return task, exists
}

// GetAll 获取所有任务信息
func (r *TaskRegistry) GetAll() map[string]*TaskInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 返回副本，避免外部修改
	result := make(map[string]*TaskInfo, len(r.tasks))
	for k, v := range r.tasks {
		result[k] = v
	}
	return result
}

// GetNames 获取所有任务名称
func (r *TaskRegistry) GetNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tasks))
	for name := range r.tasks {
		names = append(names, name)
	}
	return names
}

// Count 获取任务数量
func (r *TaskRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tasks)
}

// Unregister 注销任务（通常不需要，但提供此方法以支持动态管理）
func (r *TaskRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[name]; !exists {
		return fmt.Errorf("task %s not found", name)
	}

	delete(r.tasks, name)
	return nil
}

// Clear 清空所有任务
func (r *TaskRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tasks = make(map[string]*TaskInfo)
}
