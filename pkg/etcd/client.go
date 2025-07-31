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
	"fmt"
	"time"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Client wraps etcd client with additional functionality
type Client struct {
	*clientv3.Client
}

// Close closes the etcd client
func (c *Client) Close() error {
	return c.Client.Close()
}

// NewClient creates a new etcd client
func NewClient(endpoints []string) (*Client, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		DialTimeout:          5 * time.Second,
		DialKeepAliveTime:    30 * time.Second,
		DialKeepAliveTimeout: 5 * time.Second,
		MaxCallSendMsgSize:   2 * 1024 * 1024,
		MaxCallRecvMsgSize:   4 * 1024 * 1024,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	return &Client{Client: cli}, nil
}

// GetClusterMembers returns the list of cluster members
func (c *Client) GetClusterMembers(ctx context.Context) ([]etcdv1alpha1.EtcdMember, error) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.MemberList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}

	members := make([]etcdv1alpha1.EtcdMember, 0, len(resp.Members))
	for _, member := range resp.Members {
		etcdMember := etcdv1alpha1.EtcdMember{
			Name:  member.Name,
			ID:    fmt.Sprintf("%x", member.ID),
			Ready: true, // 如果能获取到成员信息，说明是就绪的
		}

		// 设置 PeerURL
		if len(member.PeerURLs) > 0 {
			etcdMember.PeerURL = member.PeerURLs[0]
		}

		// 设置 ClientURL
		if len(member.ClientURLs) > 0 {
			etcdMember.ClientURL = member.ClientURLs[0]
		}

		members = append(members, etcdMember)
	}

	return members, nil
}

// AddMember adds a new member to the etcd cluster
func (c *Client) AddMember(ctx context.Context, peerURL string) (*clientv3.MemberAddResponse, error) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.MemberAdd(ctx, []string{peerURL})
	if err != nil {
		return nil, fmt.Errorf("failed to add member: %w", err)
	}
	return resp, nil
}

// RemoveMember removes a member from the etcd cluster
func (c *Client) RemoveMember(ctx context.Context, memberID uint64) error {
	_, err := c.MemberRemove(ctx, memberID)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	return nil
}

// GetLeader returns the current leader information
func (c *Client) GetLeader(ctx context.Context) (string, error) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.Status(ctx, c.Endpoints()[0])
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}
	return fmt.Sprintf("%x", resp.Leader), nil
}

// HealthCheck performs a health check on the etcd cluster
func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 尝试执行一个简单的操作来检查健康状态
	_, err := c.Get(ctx, "health-check", clientv3.WithCountOnly())
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	return nil
}

// GetClusterID returns the cluster ID
func (c *Client) GetClusterID(ctx context.Context) (string, error) {
	if len(c.Endpoints()) == 0 {
		return "", fmt.Errorf("no endpoints available")
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.Status(ctx, c.Endpoints()[0])
	if err != nil {
		return "", fmt.Errorf("failed to get cluster status: %w", err)
	}
	return fmt.Sprintf("%x", resp.Header.ClusterId), nil
}

// WaitForMemberReady waits for a member to become ready
func (c *Client) WaitForMemberReady(ctx context.Context, memberName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for member %s to be ready", memberName)
		case <-ticker.C:
			members, err := c.GetClusterMembers(ctx)
			if err != nil {
				continue // 继续尝试
			}

			for _, member := range members {
				if member.Name == memberName && member.Ready {
					return nil
				}
			}
		}
	}
}
