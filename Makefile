.PHONY: all
all:
	go build -trimpath -o bin/firewall-policy-controller cmd/controller/main.go
	strip bin/firewall-policy-controller
