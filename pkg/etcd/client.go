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

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// DefaultDialTimeout etcd连接超时时间
	DefaultDialTimeout = 5 * time.Second
	// DefaultRequestTimeout etcd请求超时时间
	DefaultRequestTimeout = 5 * time.Second
)

// ListMembers 列出etcd集群中的所有成员
func ListMembers(clientURLs []string, tc *tls.Config) (*clientv3.MemberListResponse, error) {
	cfg := clientv3.Config{
		Endpoints:   clientURLs,
		DialTimeout: DefaultDialTimeout,
		TLS:         tc,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("list members failed: creating etcd client failed: %v", err)
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout)
	defer cancel()

	resp, err := etcdcli.MemberList(ctx)
	if err != nil {
		return nil, fmt.Errorf("list members failed: %v", err)
	}

	return resp, nil
}

// RemoveMember 从etcd集群中移除指定ID的成员
func RemoveMember(clientURLs []string, tc *tls.Config, id uint64) error {
	cfg := clientv3.Config{
		Endpoints:   clientURLs,
		DialTimeout: DefaultDialTimeout,
		TLS:         tc,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return fmt.Errorf("remove member failed: creating etcd client failed: %v", err)
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout)
	defer cancel()

	_, err = etcdcli.Cluster.MemberRemove(ctx, id)
	if err != nil {
		return fmt.Errorf("remove member failed: %v", err)
	}

	return nil
}

// AddMember 向etcd集群中添加新成员
func AddMember(clientURLs []string, tc *tls.Config, peerURLs []string) (*clientv3.MemberAddResponse, error) {
	cfg := clientv3.Config{
		Endpoints:   clientURLs,
		DialTimeout: DefaultDialTimeout,
		TLS:         tc,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("add member failed: creating etcd client failed: %v", err)
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout)
	defer cancel()

	resp, err := etcdcli.Cluster.MemberAdd(ctx, peerURLs)
	if err != nil {
		return nil, fmt.Errorf("add member failed: %v", err)
	}

	return resp, nil
}

// CreateClient 创建etcd客户端
func CreateClient(clientURLs []string, tc *tls.Config) (*clientv3.Client, error) {
	cfg := clientv3.Config{
		Endpoints:   clientURLs,
		DialTimeout: DefaultDialTimeout,
		TLS:         tc,
	}

	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create etcd client failed: %v", err)
	}

	return etcdcli, nil
}

// IsHealthy 检查etcd集群是否健康
func IsHealthy(clientURLs []string, tc *tls.Config) error {
	cfg := clientv3.Config{
		Endpoints:   clientURLs,
		DialTimeout: DefaultDialTimeout,
		TLS:         tc,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return fmt.Errorf("health check failed: creating etcd client failed: %v", err)
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout)
	defer cancel()

	// 尝试获取集群状态
	_, err = etcdcli.Status(ctx, clientURLs[0])
	if err != nil {
		return fmt.Errorf("health check failed: %v", err)
	}

	return nil
}
