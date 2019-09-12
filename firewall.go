package main

import (
	"bytes"
	"strings"
	"text/template"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

type MetalFirewall struct {
	c            k8s.Interface
	IngressRules []string
	EgressRules  []string
}

func NewMetalFirewall(client k8s.Interface) *MetalFirewall {
	return &MetalFirewall{
		c: client,
	}
}

func (f *MetalFirewall) AssembleRules(npl *networkingv1.NetworkPolicyList) error {
	f.IngressRules = []string{}
	f.EgressRules = []string{}
	for _, np := range npl.Items {
		hasIngress := false
		hasEgress := false
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
		if hasIngress {
			serviceName := ""
			for k, v := range np.ObjectMeta.Annotations {
				if k == NetworkPolicyAnnotationServiceName {
					serviceName = v
				}
			}
			if serviceName == "" {
				continue
			}
			svc, err := f.c.CoreV1().Services(np.ObjectMeta.Namespace).Get(serviceName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			sp := NewServicePolicy(np, *svc)
			f.IngressRules = append(f.IngressRules, sp.IngressRules()...)
		}
		if hasEgress {
			f.EgressRules = append(f.EgressRules, EgressRules(np)...)
		}
	}
	return nil
}

func (m *MetalFirewall) render() string {
	var b bytes.Buffer
	tpl := template.Must(template.New("v4").Parse(NFTABLE_TEMPLATE_V4))
	tpl.Execute(&b, m)
	return b.String()
}
