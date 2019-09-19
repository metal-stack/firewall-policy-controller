package main

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func IngressRulesNP(np networkingv1.NetworkPolicy) []string {
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
			rules = append(rules, assembleDestinationPortRule(common, "tcp", tcpPorts, comment))
		}
		if len(udpPorts) > 0 {
			rules = append(rules, assembleDestinationPortRule(common, "udp", udpPorts, comment))
		}
	}
	return rules
}

func IngressRules(svc corev1.Service) []string {
	allow := []string{}
	if len(svc.Spec.LoadBalancerSourceRanges) == 0 {
		allow = append(allow, "0.0.0.0/0")
	}
	allow = append(allow, svc.Spec.LoadBalancerSourceRanges...)
	common := []string{}
	if len(allow) > 0 {
		common = append(common, fmt.Sprintf("ip saddr { %s }", strings.Join(allow, ", ")))
	}
	common = append(common, fmt.Sprintf("ip daddr %s", svc.Spec.LoadBalancerIP))
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

func EgressRules(np networkingv1.NetworkPolicy) []string {
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
			rules = append(rules, assembleDestinationPortRule(common, "tcp", tcpPorts, comment))
		}
		if len(udpPorts) > 0 {
			rules = append(rules, assembleDestinationPortRule(common, "udp", udpPorts, comment))
		}
	}
	return rules
}

func assembleDestinationPortRule(common []string, protocol string, ports []string, comment string) string {
	parts := common
	parts = append(parts, fmt.Sprintf("%s dport { %s }", protocol, strings.Join(ports, ", ")))
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
