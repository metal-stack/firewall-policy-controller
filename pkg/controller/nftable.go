package controller

const nftableTemplateIpv4 = `create table filter
table ip filter {
	chain input {
		type filter hook input priority 0; policy drop;
		jump ct-icmp

		iif lo counter accept comment "BGP unnumbered"
		iif lan0 ip saddr 10.0.0.0/8 udp dport 4789 counter accept comment "incoming vxlan lan0"
		iif lan1 ip saddr 10.0.0.0/8 udp dport 4789 counter accept comment "incoming vxlan lan1"
		ct state new tcp dport 22 counter accept comment "incoming ssh"

		goto refuse
	}
	chain forward {
		type filter hook forward priority 0; policy drop;
		jump ct-icmp

		# dynamic ingress rules
		{{- range .IngressRules }}
		{{ . }}
		{{- end }}

		# dynamic egress rules
		{{- range .EgressRules }}
		{{ . }}
		{{- end }}

		goto refuse
	}
	chain output {
		type filter hook output priority 0; policy drop;
		jump ct-icmp

		iif lo counter accept comment "accept output required e.g. for chrony"

		goto refuse
	}
	chain ct-icmp {
		# state dependent rules
		ct state established,related counter accept comment "accept established connections"
		ct state invalid counter drop comment "drop packets with invalid ct state"

		# no ping floods
		ip protocol icmp icmp type echo-request limit rate over 10/second burst 4 packets counter drop comment "drop ping floods"

		# ICMP
		ip protocol icmp icmp type { destination-unreachable, router-solicitation, router-advertisement, time-exceeded, parameter-problem } counter accept comment "accept icmp"
	}
	chain refuse {
		counter comment "count dropped packets"
		limit rate 2/minute counter packets 1 bytes 40 log prefix "nftables-dropped: "
	}
}
table ip nat {
    chain postrouting {
        type nat hook postrouting priority 0; policy accept;
    }
}`
