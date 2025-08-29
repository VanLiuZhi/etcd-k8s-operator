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
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Member 表示etcd集群中的一个成员
type Member struct {
	Name string
	// Kubernetes命名空间，该成员运行在其中
	Namespace string
	// ID字段可以为0，表示未知ID
	// 当我们从etcd获取成员信息时知道ID，但从Kubernetes pod列表中不知道
	ID uint64

	SecurePeer   bool // 是否启用peer TLS
	SecureClient bool // 是否启用client TLS

	// ClusterDomain 集群域名，用于构建FQDN
	ClusterDomain string
}

// Addr 返回成员的地址
func (m *Member) Addr() string {
	if m.ClusterDomain != "" {
		return fmt.Sprintf("%s.%s.%s.svc.%s", m.Name, m.clusterNameFromMemberName(), m.Namespace, m.ClusterDomain)
	}
	return fmt.Sprintf("%s.%s.%s.svc.cluster.local", m.Name, m.clusterNameFromMemberName(), m.Namespace)
}

// ClientURL 返回该成员的客户端URL
func (m *Member) ClientURL() string {
	return fmt.Sprintf("%s://%s:2379", m.clientScheme(), m.Addr())
}

// clientScheme 返回客户端连接的协议方案
func (m *Member) clientScheme() string {
	if m.SecureClient {
		return "https"
	}
	return "http"
}

// peerScheme 返回peer连接的协议方案
func (m *Member) peerScheme() string {
	if m.SecurePeer {
		return "https"
	}
	return "http"
}

// ListenClientURL 返回客户端监听URL
func (m *Member) ListenClientURL() string {
	return fmt.Sprintf("%s://0.0.0.0:2379", m.clientScheme())
}

// ListenPeerURL 返回peer监听URL
func (m *Member) ListenPeerURL() string {
	return fmt.Sprintf("%s://0.0.0.0:2380", m.peerScheme())
}

// PeerURL 返回该成员的peer URL
func (m *Member) PeerURL() string {
	return fmt.Sprintf("%s://%s:2380", m.peerScheme(), m.Addr())
}

// clusterNameFromMemberName 从成员名称中提取集群名称
func (m *Member) clusterNameFromMemberName() string {
	i := strings.LastIndex(m.Name, "-")
	if i == -1 {
		panic(fmt.Sprintf("unexpected member name: %s", m.Name))
	}
	return m.Name[:i]
}

// MemberSet 表示成员集合
type MemberSet map[string]*Member

// NewMemberSet 创建新的成员集合
func NewMemberSet(ms ...*Member) MemberSet {
	res := MemberSet{}
	for _, m := range ms {
		res[m.Name] = m
	}
	return res
}

// Diff 返回s1中存在但s2中不存在的成员集合
func (ms MemberSet) Diff(other MemberSet) MemberSet {
	diff := MemberSet{}
	for n, m := range ms {
		if _, ok := other[n]; !ok {
			diff[n] = m
		}
	}
	return diff
}

// IsEqual 判断两个成员集合是否相等
// 通过检查它们是否有相同的成员集合，成员相等性仅通过Name判断
func (ms MemberSet) IsEqual(other MemberSet) bool {
	if ms.Size() != other.Size() {
		return false
	}
	for n := range ms {
		if _, ok := other[n]; !ok {
			return false
		}
	}
	return true
}

// Size 返回成员集合的大小
func (ms MemberSet) Size() int {
	return len(ms)
}

// String 返回成员集合的字符串表示
func (ms MemberSet) String() string {
	var mstring []string
	for m := range ms {
		mstring = append(mstring, m)
	}
	return strings.Join(mstring, ",")
}

// PickOne 从成员集合中选择一个成员
func (ms MemberSet) PickOne() *Member {
	for _, m := range ms {
		return m
	}
	panic("empty member set")
}

// PeerURLPairs 返回peer URL对的列表
func (ms MemberSet) PeerURLPairs() []string {
	ps := make([]string, 0)
	for _, m := range ms {
		ps = append(ps, fmt.Sprintf("%s=%s", m.Name, m.PeerURL()))
	}
	return ps
}

// Add 向成员集合中添加成员
func (ms MemberSet) Add(m *Member) {
	ms[m.Name] = m
}

// Remove 从成员集合中移除指定名称的成员
func (ms MemberSet) Remove(name string) {
	delete(ms, name)
}

// ClientURLs 返回所有成员的客户端URL列表
func (ms MemberSet) ClientURLs() []string {
	endpoints := make([]string, 0, len(ms))
	for _, m := range ms {
		endpoints = append(endpoints, m.ClientURL())
	}
	return endpoints
}

// validPeerURL 用于验证peer URL格式的正则表达式
var validPeerURL = regexp.MustCompile(`^\w+:\/\/[\w\.\-]+(:\d+)?$`)

// MemberNameFromPeerURL 从peer URL中提取成员名称
func MemberNameFromPeerURL(pu string) (string, error) {
	// url.Parse的验证很宽松，我们进行自己的验证
	if !validPeerURL.MatchString(pu) {
		return "", errors.New("invalid PeerURL format")
	}
	u, err := url.Parse(pu)
	if err != nil {
		return "", err
	}
	path := strings.Split(u.Host, ":")[0]
	name := strings.Split(path, ".")[0]
	return name, err
}
