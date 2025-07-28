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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// EtcdClientTestSuite 定义测试套件
type EtcdClientTestSuite struct {
	suite.Suite
}

// TestEtcdClientTestSuite 运行测试套件
func TestEtcdClientTestSuite(t *testing.T) {
	suite.Run(t, new(EtcdClientTestSuite))
}

// TestNewClient 测试客户端创建
func (suite *EtcdClientTestSuite) TestNewClient() {
	// 测试无效端点
	client, err := NewClient([]string{})
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), client)

	// 测试有效端点（但不连接）
	client, err = NewClient([]string{"http://localhost:2379"})
	if err != nil {
		// 如果无法创建客户端（比如网络问题），跳过测试
		suite.T().Skip("Cannot create etcd client, skipping test")
		return
	}
	assert.NotNil(suite.T(), client)
	assert.NotNil(suite.T(), client.Client)

	// 测试关闭客户端
	err = client.Close()
	assert.NoError(suite.T(), err)
}

// TestClientMethods 测试客户端方法（不需要真实的 etcd 服务器）
func (suite *EtcdClientTestSuite) TestClientMethods() {
	// 这些测试主要验证方法签名和基本逻辑
	// 实际的功能测试需要在集成测试中进行

	// 测试客户端创建
	client, err := NewClient([]string{"http://localhost:2379"})
	if err != nil {
		suite.T().Skip("Cannot create etcd client, skipping test")
		return
	}
	defer client.Close()

	// 验证客户端不为空
	assert.NotNil(suite.T(), client)
	assert.NotNil(suite.T(), client.Client)

	// 验证端点配置
	endpoints := client.Endpoints()
	assert.Contains(suite.T(), endpoints, "http://localhost:2379")
}

// TestClientConfiguration 测试客户端配置
func (suite *EtcdClientTestSuite) TestClientConfiguration() {
	endpoints := []string{
		"http://etcd-0:2379",
		"http://etcd-1:2379",
		"http://etcd-2:2379",
	}

	client, err := NewClient(endpoints)
	if err != nil {
		suite.T().Skip("Cannot create etcd client, skipping test")
		return
	}
	defer client.Close()

	// 验证端点配置
	clientEndpoints := client.Endpoints()
	for _, endpoint := range endpoints {
		assert.Contains(suite.T(), clientEndpoints, endpoint)
	}
}
