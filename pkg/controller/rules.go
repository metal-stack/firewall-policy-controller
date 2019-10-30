package controller

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// FirewallResources holds the k8s entities that serve as input for the generation of firewall rules.
type FirewallResources struct {
	NetworkPolicyList *networkingv1.NetworkPolicyList
	ServiceList       *corev1.ServiceList
}

// FirewallRules hold the nftable rules that are generated from k8s entities.
type FirewallRules struct {
	IngressRules []string
	EgressRules  []string
}

func (fr *FirewallResources) assembleRules() (*FirewallRules, error) {
	result := &FirewallRules{}
	for _, np := range fr.NetworkPolicyList.Items {
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
			result.EgressRules = append(result.EgressRules, egressRulesForNetworkPolicy(np)...)
		}
		if hasIngress {
			result.IngressRules = append(result.EgressRules, ingressRulesForNetworkPolicy(np)...)
		}
	}
	for _, svc := range fr.ServiceList.Items {
		result.IngressRules = append(result.IngressRules, ingressRulesForService(svc)...)
	}
	return result, nil
}

// Render renders the firewall rules to a string
func (r *FirewallRules) Render() (string, error) {
	var b bytes.Buffer
	tpl := template.Must(template.New("v4").Parse(nftableTemplateIpv4))
	err := tpl.Execute(&b, r)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func ingressRulesForNetworkPolicy(np networkingv1.NetworkPolicy) []string {
	ingress := np.Spec.Ingress
	if ingress == nil {
		return nil
	}
	if np.ObjectMeta.Namespace != "" {
		return nil
	}
	rules := []string{}
	for _, i := range ingress {
		allow := []string{}
		except := []string{}
		for _, f := range i.From {
			allow = append(allow, f.IPBlock.CIDR)
			except = append(except, f.IPBlock.Except...)
		}
		common := []string{}
		if len(except) > 0 {
			common = append(common, fmt.Sprintf("ip saddr != { %s }", strings.Join(except, ", ")))
		}
		if len(allow) > 0 {
			common = append(common, fmt.Sprintf("ip saddr { %s }", strings.Join(allow, ", ")))
		}
		tcpPorts := []string{}
		udpPorts := []string{}
		for _, p := range i.Ports {
			proto := proto(p.Protocol)
			if proto == "tcp" {
				tcpPorts = append(tcpPorts, fmt.Sprint(p.Port))
			} else if proto == "udp" {
				udpPorts = append(udpPorts, fmt.Sprint(p.Port))
			}
		}
		comment := fmt.Sprintf("accept traffic for k8s network policy %s", np.ObjectMeta.Name)
		if len(tcpPorts) > 0 {
			rules = append(rules, assembleDestinationPortRule(common, "tcp", tcpPorts, comment+" tcp"))
		}
		if len(udpPorts) > 0 {
			rules = append(rules, assembleDestinationPortRule(common, "udp", udpPorts, comment+" udp"))
		}
	}
	return rules
}

func ingressRulesForService(svc corev1.Service) []string {
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer && svc.Spec.Type != corev1.ServiceTypeNodePort {
		return nil
	}
	allow := []string{}
	if len(svc.Spec.LoadBalancerSourceRanges) == 0 {
		allow = append(allow, "0.0.0.0/0")
	}
	allow = append(allow, svc.Spec.LoadBalancerSourceRanges...)
	common := []string{}
	if len(allow) > 0 {
		common = append(common, fmt.Sprintf("ip saddr { %s }", strings.Join(allow, ", ")))
	}
	ips := []string{}
	if svc.Spec.LoadBalancerIP != "" {
		ips = append(ips, svc.Spec.LoadBalancerIP)
	}
	for _, e := range svc.Status.LoadBalancer.Ingress {
		ips = append(ips, e.IP)
	}
	common = append(common, fmt.Sprintf("ip daddr { %s }", strings.Join(ips, ", ")))
	tcpPorts := []string{}
	udpPorts := []string{}
	for _, p := range svc.Spec.Ports {
		proto := proto(&p.Protocol)
		if proto == "tcp" {
			tcpPorts = append(tcpPorts, fmt.Sprint(p.Port))
		} else if proto == "udp" {
			udpPorts = append(udpPorts, fmt.Sprint(p.Port))
		}
	}
	comment := fmt.Sprintf("accept traffic for k8s service %s/%s", svc.ObjectMeta.Namespace, svc.ObjectMeta.Name)
	rules := []string{}
	if len(tcpPorts) > 0 {
		rules = append(rules, assembleDestinationPortRule(common, "tcp", tcpPorts, comment))
	}
	if len(udpPorts) > 0 {
		rules = append(rules, assembleDestinationPortRule(common, "udp", udpPorts, comment))
	}
	return rules
}

func egressRulesForNetworkPolicy(np networkingv1.NetworkPolicy) []string {
	egress := np.Spec.Egress
	if egress == nil {
		return nil
	}
	rules := []string{}
	for _, e := range egress {
		tcpPorts := []string{}
		udpPorts := []string{}
		for _, p := range e.Ports {
			proto := proto(p.Protocol)
			if proto == "tcp" {
				tcpPorts = append(tcpPorts, fmt.Sprint(p.Port))
			} else if proto == "udp" {
				udpPorts = append(udpPorts, fmt.Sprint(p.Port))
			}
		}
		allow := []string{}
		except := []string{}
		for _, t := range e.To {
			if t.IPBlock == nil {
				continue
			}
			allow = append(allow, t.IPBlock.CIDR)
			except = append(except, t.IPBlock.Except...)
		}
		common := []string{}
		if len(except) > 0 {
			common = append(common, fmt.Sprintf("ip daddr != { %s }", strings.Join(except, ", ")))
		}
		if len(allow) > 0 {
			common = append(common, fmt.Sprintf("ip daddr { %s }", strings.Join(allow, ", ")))
		}
		comment := fmt.Sprintf("accept traffic for np %s", np.ObjectMeta.Name)
		if len(tcpPorts) > 0 {
			rules = append(rules, assembleDestinationPortRule(common, "tcp", tcpPorts, comment+" tcp"))
		}
		if len(udpPorts) > 0 {
			rules = append(rules, assembleDestinationPortRule(common, "udp", udpPorts, comment+" udp"))
		}
	}
	return rules
}

func assembleDestinationPortRule(common []string, protocol string, ports []string, comment string) string {
	parts := common
	parts = append(parts, fmt.Sprintf("%s dport { %s }", protocol, strings.Join(ports, ", ")))
	parts = append(parts, "counter")
	parts = append(parts, "accept")
	if comment != "" {
		parts = append(parts, "comment", fmt.Sprintf(`"%s"`, comment))
	}
	return strings.Join(parts, " ")
}

func proto(p *corev1.Protocol) string {
	proto := "tcp"
	if p != nil {
		proto = strings.ToLower(string(*p))
	}
	return proto
}
