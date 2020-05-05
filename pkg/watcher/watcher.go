package watcher

import (
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

// Watcher is the generic struct for the firewall watchers.
type Watcher struct {
	client k8s.Interface
	logger *zap.SugaredLogger
}

// ServiceWatcher watches for changes of k8s service entities.
type ServiceWatcher Watcher

// NetworkPolicyWatcher watches for changes of k8s network policy entities.
type NetworkPolicyWatcher Watcher

// NewServiceWatcher creates a new ServiceWatcher
func NewServiceWatcher(logger *zap.SugaredLogger, client k8s.Interface) *ServiceWatcher {
	return &ServiceWatcher{
		client: client,
		logger: logger,
	}
}

// Watch watches for k8s service entities and informs the res chan; is blocking.
func (w *ServiceWatcher) Watch(res chan<- bool) {
	for {
		opts := metav1.ListOptions{}
		watcher, err := w.client.CoreV1().Services(metav1.NamespaceAll).Watch(opts)
		if err != nil {
			w.logger.Errorw("could not watch for services", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}
		w.logger.Infow("watching for services")
		for range watcher.ResultChan() {
			res <- true
		}
	}
}

// NewNetworkPolicyWatcher creates a new NetworkPolicyWatcher
func NewNetworkPolicyWatcher(logger *zap.SugaredLogger, client k8s.Interface) *NetworkPolicyWatcher {
	return &NetworkPolicyWatcher{
		client: client,
		logger: logger,
	}
}

// Watch watches for k8s network policy entities and informs the res chan; is blocking.
func (w *NetworkPolicyWatcher) Watch(res chan<- bool) {
	for {
		opts := metav1.ListOptions{}
		watcher, err := w.client.NetworkingV1().NetworkPolicies(metav1.NamespaceAll).Watch(opts)
		if err != nil {
			w.logger.Errorw("could not watch for network policies", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}
		w.logger.Infow("watching for network policies")
		for range watcher.ResultChan() {
			res <- true
		}
	}
}
