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

package testutil

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/etcd-lz/etcd-k8s-operator/pkg/etcd"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// MockEtcdClient 模拟Etcd客户端，用于测试成员管理操作
type MockEtcdClient struct {
	mu sync.RWMutex

	// 模拟的成员集合
	Members map[uint64]*etcd.Member
	// 模拟的成员状态
	MemberStates map[uint64]MemberState

	// 模拟配置
	QuorumLost       bool
	AddMemberFail    bool
	RemoveMemberFail bool
	ListMembersFail  bool
	ClientTimeout    time.Duration

	// 操作记录
	AddMemberCalls    []AddMemberCall
	RemoveMemberCalls []RemoveMemberCall
	ListMembersCalls  []ListMembersCall
}

// MemberState 成员状态
type MemberState struct {
	ID         uint64
	Name       string
	PeerURLs   []string
	ClientURLs []string
	IsLearner  bool
	Started    bool
}

// AddMemberCall 记录添加成员调用
type AddMemberCall struct {
	PeerURLs  []string
	Timestamp time.Time
	Success   bool
	Error     error
}

// RemoveMemberCall 记录移除成员调用
type RemoveMemberCall struct {
	MemberID  uint64
	Timestamp time.Time
	Success   bool
	Error     error
}

// ListMembersCall 记录列出成员调用
type ListMembersCall struct {
	Timestamp time.Time
	Success   bool
	Error     error
}

// NewMockEtcdClient 创建新的Mock Etcd客户端
func NewMockEtcdClient() *MockEtcdClient {
	return &MockEtcdClient{
		Members:           make(map[uint64]*etcd.Member),
		MemberStates:      make(map[uint64]MemberState),
		ClientTimeout:     5 * time.Millisecond, // 减少默认超时提高测试速度
		AddMemberCalls:    make([]AddMemberCall, 0),
		RemoveMemberCalls: make([]RemoveMemberCall, 0),
		ListMembersCalls:  make([]ListMembersCall, 0),
	}
}

// CreateClient 创建etcd客户端
func (m *MockEtcdClient) CreateClient(endpoints []string, tlsConfig *tls.Config) (*clientv3.Client, error) {
	if m.ClientTimeout > 0 {
		time.Sleep(m.ClientTimeout)
	}

	// 返回一个简单的Mock客户端
	return &clientv3.Client{}, nil
}

// AddMember 添加成员到etcd集群
func (m *MockEtcdClient) AddMember(endpoints []string, tlsConfig *tls.Config, peerURLs []string) (*clientv3.MemberAddResponse, error) {
	call := AddMemberCall{
		PeerURLs:  peerURLs,
		Timestamp: time.Now(),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.AddMemberFail {
		call.Success = false
		call.Error = fmt.Errorf("mock add member failed")
		m.AddMemberCalls = append(m.AddMemberCalls, call)
		return nil, call.Error
	}

	if m.QuorumLost {
		call.Success = false
		call.Error = fmt.Errorf("etcd cluster has lost quorum")
		m.AddMemberCalls = append(m.AddMemberCalls, call)
		return nil, call.Error
	}

	// 生成新成员ID
	newID := uint64(len(m.Members) + 1)

	// 创建新成员状态
	memberState := MemberState{
		ID:         newID,
		PeerURLs:   peerURLs,
		ClientURLs: []string{fmt.Sprintf("http://10.244.0.%d:2379", newID)},
		IsLearner:  true,
		Started:    false,
	}
	m.MemberStates[newID] = memberState

	// 创建对应的etcd.Member对象
	member := &etcd.Member{
		ID:           newID,
		Name:         fmt.Sprintf("etcd-%d", newID),
		Namespace:    "default",
		SecurePeer:   false,
		SecureClient: false,
	}
	m.Members[newID] = member

	call.Success = true
	m.AddMemberCalls = append(m.AddMemberCalls, call)

	// 模拟成员启动过程
	go m.simulateMemberStart(newID)

	return &clientv3.MemberAddResponse{
		Member: &etcdserverpb.Member{
			ID:         newID,
			PeerURLs:   peerURLs,
			ClientURLs: memberState.ClientURLs,
		},
	}, nil
}

// RemoveMember 从etcd集群移除成员
func (m *MockEtcdClient) RemoveMember(endpoints []string, tlsConfig *tls.Config, memberID uint64) error {
	call := RemoveMemberCall{
		MemberID:  memberID,
		Timestamp: time.Now(),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.RemoveMemberFail {
		call.Success = false
		call.Error = fmt.Errorf("mock remove member failed")
		m.RemoveMemberCalls = append(m.RemoveMemberCalls, call)
		return call.Error
	}

	if m.QuorumLost {
		call.Success = false
		call.Error = fmt.Errorf("etcd cluster has lost quorum")
		m.RemoveMemberCalls = append(m.RemoveMemberCalls, call)
		return call.Error
	}

	if _, exists := m.Members[memberID]; !exists {
		call.Success = false
		call.Error = fmt.Errorf("member %d not found", memberID)
		m.RemoveMemberCalls = append(m.RemoveMemberCalls, call)
		return call.Error
	}

	// 检查法定人数
	remainingMembers := len(m.Members) - 1
	if remainingMembers < len(m.Members)/2+1 {
		call.Success = false
		call.Error = fmt.Errorf("removing member would lose quorum")
		m.RemoveMemberCalls = append(m.RemoveMemberCalls, call)
		return call.Error
	}

	delete(m.Members, memberID)
	delete(m.MemberStates, memberID)

	call.Success = true
	m.RemoveMemberCalls = append(m.RemoveMemberCalls, call)
	return nil
}

// ListMembers 列出etcd集群成员
func (m *MockEtcdClient) ListMembers(endpoints []string, tlsConfig *tls.Config) (*clientv3.MemberListResponse, error) {
	call := ListMembersCall{
		Timestamp: time.Now(),
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.ListMembersFail {
		call.Success = false
		call.Error = fmt.Errorf("mock list members failed")
		m.ListMembersCalls = append(m.ListMembersCalls, call)
		return nil, call.Error
	}

	members := make([]*etcdserverpb.Member, 0, len(m.Members))
	for _, member := range m.Members {
		if state, exists := m.MemberStates[member.ID]; exists {
			members = append(members, &etcdserverpb.Member{
				ID:         member.ID,
				Name:       member.Name,
				PeerURLs:   state.PeerURLs,
				ClientURLs: state.ClientURLs,
			})
		}
	}

	call.Success = true
	m.ListMembersCalls = append(m.ListMembersCalls, call)

	return &clientv3.MemberListResponse{
		Members: members,
	}, nil
}

// UpdateMember 更新成员信息
func (m *MockEtcdClient) UpdateMember(endpoints []string, tlsConfig *tls.Config, memberID uint64, peerURLs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.MemberStates[memberID]; exists {
		state.PeerURLs = peerURLs
		m.MemberStates[memberID] = state
		return nil
	}
	return fmt.Errorf("member %d not found", memberID)
}

// simulateMemberStart 模拟成员启动过程
func (m *MockEtcdClient) simulateMemberStart(memberID uint64) {
	time.Sleep(200 * time.Millisecond) // 模拟启动时间

	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.MemberStates[memberID]; exists {
		state.Started = true
		state.IsLearner = false // 启动后成为正式成员
		m.MemberStates[memberID] = state
	}
}

// GetMemberState 获取成员状态
func (m *MockEtcdClient) GetMemberState(memberID uint64) (MemberState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.MemberStates[memberID]
	return state, exists
}

// GetAllMemberStates 获取所有成员状态
func (m *MockEtcdClient) GetAllMemberStates() map[uint64]MemberState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make(map[uint64]MemberState)
	for id, state := range m.MemberStates {
		states[id] = state
	}
	return states
}

// GetMemberCount 获取成员数量
func (m *MockEtcdClient) GetMemberCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.Members)
}

// GetQuorumSize 获取法定人数大小
func (m *MockEtcdClient) GetQuorumSize() int {
	memberCount := m.GetMemberCount()
	return memberCount/2 + 1
}

// HasQuorum 检查是否有法定人数
func (m *MockEtcdClient) HasQuorum() bool {
	if m.QuorumLost {
		return false
	}

	// 计算已启动的成员数量
	startedCount := 0
	m.mu.RLock()
	for _, state := range m.MemberStates {
		if state.Started {
			startedCount++
		}
	}
	m.mu.RUnlock()

	return startedCount >= m.GetQuorumSize()
}

// SetQuorumLost 设置法定人数丢失状态
func (m *MockEtcdClient) SetQuorumLost(lost bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.QuorumLost = lost
}

// SetAddMemberFail 设置添加成员失败
func (m *MockEtcdClient) SetAddMemberFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.AddMemberFail = fail
}

// SetRemoveMemberFail 设置移除成员失败
func (m *MockEtcdClient) SetRemoveMemberFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RemoveMemberFail = fail
}

// ClearCallRecords 清除调用记录
func (m *MockEtcdClient) ClearCallRecords() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.AddMemberCalls = make([]AddMemberCall, 0)
	m.RemoveMemberCalls = make([]RemoveMemberCall, 0)
	m.ListMembersCalls = make([]ListMembersCall, 0)
}

// GetCallStats 获取调用统计
func (m *MockEtcdClient) GetCallStats() (addCount, removeCount, listCount int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.AddMemberCalls), len(m.RemoveMemberCalls), len(m.ListMembersCalls)
}

// GetLastAddMemberCall 获取最后一次添加成员调用
func (m *MockEtcdClient) GetLastAddMemberCall() *AddMemberCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.AddMemberCalls) == 0 {
		return nil
	}
	return &m.AddMemberCalls[len(m.AddMemberCalls)-1]
}

// GetLastRemoveMemberCall 获取最后一次移除成员调用
func (m *MockEtcdClient) GetLastRemoveMemberCall() *RemoveMemberCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.RemoveMemberCalls) == 0 {
		return nil
	}
	return &m.RemoveMemberCalls[len(m.RemoveMemberCalls)-1]
}

// SimulateMemberFailure 模拟成员故障
func (m *MockEtcdClient) SimulateMemberFailure(memberID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.MemberStates[memberID]; exists {
		state.Started = false
		m.MemberStates[memberID] = state
	}
}

// SimulateMemberRecovery 模拟成员恢复
func (m *MockEtcdClient) SimulateMemberRecovery(memberID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.MemberStates[memberID]; exists {
		state.Started = true
		m.MemberStates[memberID] = state
	}
}

// GetStartedMembers 获取已启动的成员列表
func (m *MockEtcdClient) GetStartedMembers() []*etcd.Member {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var started []*etcd.Member
	for _, member := range m.Members {
		if state, exists := m.MemberStates[member.ID]; exists && state.Started {
			started = append(started, member)
		}
	}
	return started
}

// ValidateClusterState 验证集群状态是否有效
func (m *MockEtcdClient) ValidateClusterState() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查是否有成员
	if len(m.Members) == 0 {
		return fmt.Errorf("no members in cluster")
	}

	// 检查法定人数
	if !m.HasQuorum() {
		return fmt.Errorf("cluster has lost quorum")
	}

	// 检查所有成员的状态
	for _, member := range m.Members {
		state, exists := m.MemberStates[member.ID]
		if !exists {
			return fmt.Errorf("member %d has no state", member.ID)
		}
		if len(state.PeerURLs) == 0 {
			return fmt.Errorf("member %d has no peer URLs", member.ID)
		}
	}

	return nil
}

// PrintClusterState 打印集群状态（用于调试）
func (m *MockEtcdClient) PrintClusterState() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := fmt.Sprintf("MockEtcdClient State:\n")
	result += fmt.Sprintf("  Members: %d\n", len(m.Members))
	result += fmt.Sprintf("  Quorum Lost: %v\n", m.QuorumLost)
	result += fmt.Sprintf("  Has Quorum: %v\n", m.HasQuorum())
	result += fmt.Sprintf("  Quorum Size: %d\n", m.GetQuorumSize())

	result += "  Member States:\n"
	for id, state := range m.MemberStates {
		result += fmt.Sprintf("    Member %d: Started=%v, IsLearner=%v, PeerURLs=%v\n",
			id, state.Started, state.IsLearner, state.PeerURLs)
	}

	return result
}
