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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/etcd-lz/etcd-k8s-operator/pkg/testutil"
)

var (
	clusterName = flag.String("cluster-name", "debug-etcd-cluster", "Name of the etcd cluster")
	namespace   = flag.String("namespace", "default", "Namespace for the cluster")
	initialSize = flag.Int("size", 3, "Initial cluster size")
	debugMode   = flag.Bool("debug", true, "Enable debug logging")
	timeout     = flag.Duration("timeout", 60*time.Second, "Operation timeout")
	interactive = flag.Bool("interactive", true, "Enable interactive mode")
)

func main() {
	flag.Parse()

	// 创建测试配置
	config := &testutil.TestConfig{
		ClusterName:      *clusterName,
		Namespace:        *namespace,
		InitialSize:      int32(*initialSize),
		ReconcileTimeout: *timeout,
		EnableDebugLog:   *debugMode,
	}

	// 创建测试集群
	testCluster := testutil.NewTestCluster(nil, config)
	testCluster.Start()
	defer testCluster.Stop()

	log.Printf("=== Etcd Cluster Reconciler Debug Tool ===")
	log.Printf("Cluster: %s/%s", config.Namespace, config.ClusterName)
	log.Printf("Initial Size: %d", config.InitialSize)
	log.Printf("Debug Mode: %v", config.EnableDebugLog)
	log.Printf("=====================================")

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待集群就绪
	log.Printf("Waiting for cluster to be ready...")
	err := testCluster.WaitForClusterReady(*timeout)
	if err != nil {
		log.Fatalf("Cluster failed to become ready: %v", err)
	}
	log.Printf("Cluster is ready!")

	// 打印初始状态
	testCluster.PrintClusterState()

	if *interactive {
		runInteractiveMode(testCluster)
	} else {
		runBasicScenario(testCluster)
	}

	log.Printf("Debug session completed")
}

func runInteractiveMode(tc *testutil.TestCluster) {
	log.Printf("=== Interactive Mode ===")
	log.Printf("Commands:")
	log.Printf("  status    - Show cluster status")
	log.Printf("  add <n>   - Add N members")
	log.Printf("  remove <n> - Remove N members")
	log.Printf("  fail <pod> - Simulate pod failure")
	log.Printf("  recover <pod> - Simulate pod recovery")
	log.Printf("  quorum    - Toggle quorum loss")
	log.Printf("  help      - Show this help")
	log.Printf("  quit      - Exit")
	log.Printf("=========================")

	for {
		var cmd string
		fmt.Print("> ")
		fmt.Scanln(&cmd)

		switch cmd {
		case "status":
			tc.PrintClusterState()
		case "help":
			log.Printf("Available commands: status, add, remove, fail, recover, quorum, quit")
		case "quit", "exit":
			return
		case "quorum":
			current := tc.MockEtcd.HasQuorum()
			tc.MockEtcd.SetQuorumLost(!current)
			log.Printf("Quorum loss set to: %v", !current)
		default:
			log.Printf("Unknown command: %s", cmd)
			log.Printf("Type 'help' for available commands")
		}
	}
}

func runBasicScenario(tc *testutil.TestCluster) {
	log.Printf("=== Running Basic Scenario ===")

	// 等待一段时间观察调谐
	time.Sleep(10 * time.Second)

	// 模拟添加成员
	log.Printf("Adding a new member...")
	_, err := tc.MockEtcd.AddMember([]string{"http://localhost:2380"}, nil, []string{"http://new-member:2380"})
	if err != nil {
		log.Printf("Failed to add member: %v", err)
	} else {
		log.Printf("Member added successfully")
	}

	// 等待调谐
	time.Sleep(5 * time.Second)

	// 模拟移除成员
	log.Printf("Removing a member...")
	members := tc.MockEtcd.GetAllMemberStates()
	if len(members) > 0 {
		for id := range members {
			err := tc.MockEtcd.RemoveMember([]string{"http://localhost:2380"}, nil, id)
			if err != nil {
				log.Printf("Failed to remove member %d: %v", id, err)
			} else {
				log.Printf("Member %d removed successfully", id)
				break
			}
		}
	}

	// 等待最终调谐
	time.Sleep(10 * time.Second)

	// 打印最终状态
	log.Printf("=== Final State ===")
	tc.PrintClusterState()
}
