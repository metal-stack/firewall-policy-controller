package main

import (
	"bytes"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

type MetalFirewall struct {
	c            k8s.Interface
	IngressRules []string
	EgressRules  []string
}

type Input struct {
	NetworkPolicyList networkingv1.NetworkPolicyList
	ServiceList       corev1.ServiceList
}

func NewMetalFirewall(client k8s.Interface) *MetalFirewall {
	return &MetalFirewall{
		c: client,
	}
}

func (f *MetalFirewall) AssembleRules() error {
	npl, err := f.c.NetworkingV1().NetworkPolicies(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	f.IngressRules = []string{}
	f.EgressRules = []string{}
	for _, np := range npl.Items {
		hasEgress := false
		hasIngress := false
		for _, pt := range np.Spec.PolicyTypes {
			switch strings.ToLower(string(pt)) {
			case "ingress":
				hasIngress = true
			case "egress":
				hasEgress = true
			case "both":
				hasIngress = true
				hasEgress = true
			}
		}
		if hasEgress {
			f.EgressRules = append(f.EgressRules, EgressRules(np)...)
		}
		if hasIngress {
			f.IngressRules = append(f.EgressRules, IngressRulesNP(np)...)
		}
	}
	svcs, err := f.c.CoreV1().Services(metav1.NamespaceAll).List(metav1.ListOptions{})
	for _, svc := range svcs.Items {
		f.IngressRules = append(f.IngressRules, IngressRules(svc)...)
	}
	return nil
}

func (m *MetalFirewall) render() string {
	var b bytes.Buffer
	tpl := template.Must(template.New("v4").Parse(NFTABLE_TEMPLATE_V4))
	tpl.Execute(&b, m)
	return b.String()
}
