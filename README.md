# Firewall-Policy-Controller

This is a small controller to generate nftables rules based on network policies and services.

## Current scope for the implementation
- the firewall is not part of the kubernetes cluster
    => is not visible as node and gets no pods scheduled on it
- it gets access to the kube-api server with a kubeconfig that gets injected via ignition user data
- it watches for `NetworkPolicy` objects in the default namespace  and `Service` objects in all namespaces and assembles ingress / egress firewall rules for them
    - `NetworkPolicy` need an empty `podSelector`
    - `Service` objects of type `LoadBalancer` and `NodePort` need the `loadBalancerSourceRanges` attribute

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: np-egress-dns
  namespace: default
spec:
  podSelector: {}
  policyTypes:
  - Egress
  egress:
  - to:
    - ipBlock:
        cidr: 1.0.0.1/32
    ports:
    - protocol: UDP
      port: 53

```

```yaml
apiVersion: v1
kind: Service
metadata:
  name: s1
  namespace: test-ns
spec:
  type: LoadBalancer
  loadBalancerIP: 212.37.83.1
  loadBalancerSourceRanges:
  - 192.168.0.0/24
  ports:
  - name: http
    protocol: TCP
    port: 80
    targetPort: 8063
```

## Testing locally

```
make
./bin/firewall-policy-controller -k kubeconfig
kubectl --kubeconfig kubeconfig apply --recursive -f pkg/controller/test_data/case1/
kubectl --kubeconfig kubeconfig delete --recursive -f pkg/controller/test_data/case1/
```
