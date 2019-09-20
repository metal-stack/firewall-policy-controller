# Metal-Firewall

This sould enforce network policies on our firewalls.

This is work in progress and is likely to be merged into the metal-networker.

## Current scope for the implementation
- the firewall is not part of the kubernetes cluster
    => is not visible as node and gets no pods scheduled on it
- it gets access to the kube-api server with a kubeconfig that gets injected via cloud.init user data
- only enforce policies with empty `podSelector`

## Testing locally

```
./bin/firewall-policy-controller -k kubeconfig
kubectl --kubeconfig kubeconfig apply --recursive -f pkg/controller/test_data/case1/
kubectl --kubeconfig kubeconfig delete --recursive -f pkg/controller/test_data/case1/
```