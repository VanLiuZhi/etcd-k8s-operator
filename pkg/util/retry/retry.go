/*
Copyright 2025 ETCD Operator Team.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package retry

import (
	"fmt"
	"time"
)

// RetryError 重试错误
type RetryError struct {
	Attempts int
	LastErr  error
}

func (e *RetryError) Error() string {
	return fmt.Sprintf("still failing after %d retries, last error: %v", e.Attempts, e.LastErr)
}

// IsRetryError 检查是否为重试错误
func IsRetryError(err error) bool {
	_, ok := err.(*RetryError)
	return ok
}

// ConditionFunc 条件函数类型
type ConditionFunc func() (bool, error)

// Retry 重试执行条件函数，直到成功或达到最大重试次数
// interval 重试间隔
// maxRetries 最大重试次数
// f 条件函数，返回 (是否完成, 错误)
func Retry(interval time.Duration, maxRetries int, f ConditionFunc) error {
	if maxRetries <= 0 {
		return fmt.Errorf("maxRetries (%d) should be > 0", maxRetries)
	}

	var lastErr error

	for i := 0; i < maxRetries; i++ {
		done, err := f()
		if err != nil {
			lastErr = err
			if i == maxRetries-1 {
				break
			}
			time.Sleep(interval)
			continue
		}

		if done {
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(interval)
		}
	}

	return &RetryError{
		Attempts: maxRetries,
		LastErr:  lastErr,
	}
}

// RetryWithBackoff 重试执行，支持指数退避
// initialInterval 初始间隔
// maxRetries 最大重试次数
// f 条件函数
func RetryWithBackoff(initialInterval time.Duration, maxRetries int, f ConditionFunc) error {
	if maxRetries <= 0 {
		return fmt.Errorf("maxRetries (%d) should be > 0", maxRetries)
	}

	var lastErr error

	for i := 0; i < maxRetries; i++ {
		done, err := f()
		if err != nil {
			lastErr = err
			if i == maxRetries-1 {
				break
			}
			// 指数退避：初始间隔 * 2^i
			sleepTime := initialInterval * time.Duration(1<<uint(i))
			time.Sleep(sleepTime)
			continue
		}

		if done {
			return nil
		}

		if i < maxRetries-1 {
			sleepTime := initialInterval * time.Duration(1<<uint(i))
			time.Sleep(sleepTime)
		}
	}

	return &RetryError{
		Attempts: maxRetries,
		LastErr:  lastErr,
	}
}