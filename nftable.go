package main

const NFTABLE_TEMPLATE_V4 = `create table mf
table ip mf {
	chain mf-input {
		type filter hook input priority 0;
		policy drop

		jump mf-ct-icmp

		iif lo accept comment "BGP unnumbered"
		iif lan0 ip saddr 10.0.0.0/8 udp dport 4789 accept comment "incoming vxlan lan0"
		iif lan1 ip saddr 10.0.0.0/8 udp dport 4789 accept comment "incoming vxlan lan1"
		ct state new tcp dport 22 accept comment "incoming ssh"

		goto mf-final
	}
	chain mf-forward {
		type filter hook forward priority 0;
		policy drop

		jump mf-ct-icmp

		# dynamic ingress rules
		{{- range .IngressRules }}
		{{ . }}
		{{- end }}

		# dynamic egress rules
		{{- range .EgressRules }}
		{{ . }}
		{{- end }}

		goto mf-final
	}
	chain mf-ct-output {
		type filter hook input priority 0;
		policy drop

		jump mf-ct-icmp

		iif lo accept comment "accept output required e.g. for chrony"

		goto mf-final
	}
	chain mf-ct-icmp {
		# state dependent rules
		ct state established,related accept comment "accept established connections"
		ct state invalid drop comment "drop packets with invalid ct state"

		# no ping floods
		ip protocol icmp icmp type echo-request limit rate over 10/second burst 4 packets drop comment "drop ping floods"

		# ICMP & IGMP
		ip protocol icmp icmp type { destination-unreachable, router-solicitation, router-advertisement, time-exceeded, parameter-problem } accept comment "accept icmp"
		ip protocol igmp accept comment "accept igmp"

		goto mf-final
	}
	chain mf-final {
		counter comment "count dropped packets"
		log prefix "dropped: "
	}
}
`
