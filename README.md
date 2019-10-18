# Metal-Firewall

This sould enforce network policies on our firewalls.

This is work in progress and is likely to be merged into the metal-networker.

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

## Testing droptailer

```bash
# Create certificates
mkdir certs
cd certs
echo '{"CN":"CA","key":{"algo":"rsa","size":2048}}' | cfssl gencert -initca - | cfssljson -bare ca -
echo '{"signing":{"default":{"expiry":"43800h","usages":["signing","key encipherment","server auth","client auth"]}}}' > ca-config.json
export ADDRESS=droptailer
export NAME=droptailer-server
echo '{"CN":"'$NAME'","hosts":[""],"key":{"algo":"rsa","size":2048}}' \
    | cfssl gencert -config=ca-config.json -ca=ca.pem -ca-key=ca-key.pem -hostname="$ADDRESS" - \
    | cfssljson -bare $NAME
export ADDRESS=
export NAME=droptailer-client
echo '{"CN":"'$NAME'","hosts":[""],"key":{"algo":"rsa","size":2048}}' \
    | cfssl gencert -config=ca-config.json -ca=ca.pem -ca-key=ca-key.pem -hostname="$ADDRESS" - \
    | cfssljson -bare $NAME
cd -

# Create kind cluster and start firewall-policy-controller
kind create cluster
export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
export CERTIFICATE_BASE=./certs/
./bin/firewall-policy-controller -k /home/markus/.kube/kind-config-kind

# Expose droptailer-server port to host
podName=$(kubectl get pods -n firewall -o=jsonpath='{.items[0].metadata.name}')
kubectl port-forward -n firewall --address 0.0.0.0 pod/$podName 50051:50051 &

# Run droptailer-client
docker run -it \
  --privileged \
  --add-host droptailer:172.17.0.1 \
  --env DROPTAILER_SERVER_ADDRESS=droptailer:50051 \
  --env DROPTAILER_CA_CERTIFICATE=/certs/ca.pem \
  --env DROPTAILER_CLIENT_CERTIFICATE=/certs/droptailer-client.pem \
  --env DROPTAILER_CLIENT_KEY=/certs/droptailer-client-key.pem \
  --volume $(pwd)/certs:/certs \
  --volume /run/systemd/private:/run/systemd/private \
  --volume /var/log/journal:/var/log/journal \
  --volume /run/log/journal:/run/log/journal \
  --volume /etc/machine-id:/etc/machine-id \
metalpod/droptailer-client

# Watch for drops at the firewall
stern -n firewall drop

# Generate a sample message for the systemd journal that gets catched by the droptailer-client
sudo logger -t kernel "nftables-metal-dropped: IN=vrf09 OUT= MAC=12:99:fd:3b:ce:f8:1a:ae:e9:a7:95:50:08:00 SRC=222.73.197.30 DST=212.34.89.87 LEN=40 TOS=0x00 PREC=0x00 TTL=238 ID=46474 PROTO=TCP SPT=59265 DPT=445 WINDOW=1024 RES=0x00 SYN URGP=0"
```