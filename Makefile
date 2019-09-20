BINARY := firewall-policy-controller
MAINMODULE := git.f-i-ts.de/cloud-native/firewall-policy-controller/cmd/controller
COMMONDIR := $(or ${COMMONDIR},../common)

include $(COMMONDIR)/Makefile.inc