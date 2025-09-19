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
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// EventRecord 记录K8s事件
type EventRecord struct {
	EventType string
	Reason    string
	Message   string
	Timestamp time.Time
}

// SimpleK8sClient 简化的K8s客户端，只实现测试需要的基本功能
type SimpleK8sClient struct {
	mu sync.RWMutex

	Pods     map[string]*corev1.Pod
	Services map[string]*corev1.Service

	// 模拟配置
	StateChangeDelay time.Duration
	CreatePodFail    bool

	EventRecords []EventRecord
}

// NewSimpleK8sClient 创建简化的K8s客户端
func NewSimpleK8sClient() *SimpleK8sClient {
	return &SimpleK8sClient{
		Pods:             make(map[string]*corev1.Pod),
		Services:         make(map[string]*corev1.Service),
		StateChangeDelay: 10 * time.Millisecond,
		CreatePodFail:    false,
		EventRecords:     make([]EventRecord, 0),
	}
}

// CreatePod 创建Pod
func (c *SimpleK8sClient) CreatePod(ctx context.Context, namespace string, pod *corev1.Pod) error {
	if c.CreatePodFail {
		return fmt.Errorf("mock pod creation failed")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, pod.Name)

	// 创建Pod副本
	podCopy := pod.DeepCopy()
	podCopy.Status.Phase = corev1.PodPending
	podCopy.Status.PodIP = "10.244.0.100"

	c.Pods[key] = podCopy

	// 模拟Pod状态变化
	go c.simulatePodStateChange(key)

	return nil
}

// GetPod 获取Pod
func (c *SimpleK8sClient) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	pod, exists := c.Pods[key]
	if !exists {
		return nil, fmt.Errorf("pod not found")
	}
	return pod.DeepCopy(), nil
}

// ListPods 列出Pod
func (c *SimpleK8sClient) ListPods(ctx context.Context, namespace string) ([]*corev1.Pod, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var pods []*corev1.Pod
	for key, pod := range c.Pods {
		if namespace == "" || key[:len(namespace)] == namespace {
			pods = append(pods, pod.DeepCopy())
		}
	}
	return pods, nil
}

// DeletePod 删除Pod
func (c *SimpleK8sClient) DeletePod(ctx context.Context, namespace, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	if _, exists := c.Pods[key]; !exists {
		return fmt.Errorf("pod not found")
	}
	delete(c.Pods, key)
	return nil
}

// CreateService 创建Service
func (c *SimpleK8sClient) CreateService(ctx context.Context, namespace string, svc *corev1.Service) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, svc.Name)
	c.Services[key] = svc.DeepCopy()
	return nil
}

// GetService 获取Service
func (c *SimpleK8sClient) GetService(ctx context.Context, namespace, name string) (*corev1.Service, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	svc, exists := c.Services[key]
	if !exists {
		return nil, fmt.Errorf("service not found")
	}
	return svc.DeepCopy(), nil
}

// simulatePodStateChange 模拟Pod状态变化
func (c *SimpleK8sClient) simulatePodStateChange(podKey string) {
	time.Sleep(c.StateChangeDelay)

	c.mu.Lock()
	defer c.mu.Unlock()

	if pod, exists := c.Pods[podKey]; exists {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.PodIPs = []corev1.PodIP{
			{IP: pod.Status.PodIP},
		}
		pod.Status.Conditions = []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		}
	}
}

// SetPodState 手动设置Pod状态
func (c *SimpleK8sClient) SetPodState(namespace, name string, phase corev1.PodPhase) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	if pod, exists := c.Pods[key]; exists {
		pod.Status.Phase = phase
		if phase == corev1.PodRunning {
			pod.Status.PodIP = "10.244.0.100"
			pod.Status.Conditions = []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			}
		}
	}
}

// GetPodState 获取Pod状态统计
func (c *SimpleK8sClient) GetPodState() (running, pending int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, pod := range c.Pods {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			running++
		case corev1.PodPending:
			pending++
		}
	}
	return
}

// RecordEvent 记录事件
func (c *SimpleK8sClient) RecordEvent(eventType, reason, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.EventRecords = append(c.EventRecords, EventRecord{
		EventType: eventType,
		Reason:    reason,
		Message:   message,
		Timestamp: time.Now(),
	})
}

// GetEventRecords 获取事件记录
func (c *SimpleK8sClient) GetEventRecords() []EventRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	records := make([]EventRecord, len(c.EventRecords))
	copy(records, c.EventRecords)
	return records
}

// ClearEventRecords 清除事件记录
func (c *SimpleK8sClient) ClearEventRecords() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.EventRecords = make([]EventRecord, 0)
}