.PHONY: all
all:
	go test -v -cover ./...
	go build -trimpath -o bin/firewall-policy-controller main.go
	strip bin/firewall-policy-controller
