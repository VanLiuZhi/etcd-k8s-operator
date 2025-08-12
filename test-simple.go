package main

import (
	"fmt"

	"github.com/your-org/etcd-k8s-operator/internal/controller"
)

func main() {
	fmt.Println("SIMPLE-TEST-12345-UNIQUE-STRING")
	r := &controller.EtcdClusterReconciler{}
	fmt.Printf("Reconciler: %v\n", r)
}
