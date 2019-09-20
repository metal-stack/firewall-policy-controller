package controller

import (
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

// FirewallController watches for changes of the k8s entities services and networkpolicies and constructs nftable rules for them.
type FirewallController struct {
	c      k8s.Interface
	logger *zap.SugaredLogger
}

// NewFirewallController creates a new FirewallController
func NewFirewallController(client k8s.Interface, logger *zap.SugaredLogger) *FirewallController {
	return &FirewallController{
		c:      client,
		logger: logger,
	}
}

// FetchAndAssemble fetches resources from k8s and assembles firewall rules for them
func (f *FirewallController) FetchAndAssemble() (*FirewallRules, error) {
	r, err := f.fetchResouces()
	if err != nil {
		return nil, err
	}
	rules, err := r.assembleRules()
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (f *FirewallController) fetchResouces() (*FirewallResources, error) {
	npl, err := f.c.NetworkingV1().NetworkPolicies(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	svcs, err := f.c.CoreV1().Services(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return &FirewallResources{
		NetworkPolicyList: npl,
		ServiceList:       svcs,
	}, nil
}
