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

package etcd

import "errors"

var (
	// ErrLostQuorum 表示etcd集群失去法定人数
	ErrLostQuorum = errors.New("etcd cluster lost quorum")

	// ErrMemberNotFound 表示找不到指定的成员
	ErrMemberNotFound = errors.New("etcd member not found")

	// ErrClusterNotHealthy 表示etcd集群不健康
	ErrClusterNotHealthy = errors.New("etcd cluster is not healthy")

	// ErrInvalidMemberName 表示成员名称无效
	ErrInvalidMemberName = errors.New("invalid etcd member name")

	// ErrInvalidClusterSize 表示集群大小无效
	ErrInvalidClusterSize = errors.New("invalid etcd cluster size")
)

// IsFatalError 判断是否为致命错误
func IsFatalError(err error) bool {
	return err == ErrLostQuorum
}
