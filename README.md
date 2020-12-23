# envoy-apisix-in-mesh

This project aims to demonstrate how to run [envoy-apisix](https://github.com/api7/envoy-apisix) in Istio mesh.

## Kubernetes Cluster & Service Mesh

Before you start, make sure you have an available Kubernetes cluster, [Minikube](https://minikube.sigs.k8s.io/docs/start/) is a nice choice to build Kubernetes cluster in your own development environment.

Also, we need [Istio distribution](https://github.com/istio/istio/releases/tag/1.8.1) to build the Service Mesh. So Istio ingress/egress gateways, Istiod and other resources should also be installed, use Istio's helm [charts](https://github.com/istio/istio/tree/master/manifests/charts) to install will make the process easiler.

## Topology

We will use the [Bookinfo Application](https://istio.io/latest/docs/examples/bookinfo/) as the base Service Mesh environment. The topology as shown below:

![ ](https://istio.io/latest/docs/examples/bookinfo/withistio.svg)

Install all the necessary apps by running:

```
$ kubectl apply -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo.yaml -n bookinfo
```

Then, add a VirtualService resource to configure the istio-gateway, so that it knows how to forward trafic to the mesh. Note we have our own [copy](./samples/bookinfo-gateway.yaml) of it with some minor changes (use FQDN).

```
$ kubectl apply -f https://raw.githubusercontent.com/tokers/envoy-apisix-in-mesh/main/samples/bookinfo-gateway.yaml -n istio-system
```

## Extra Steps

Although Envoy embeds LuaJIT to extend itself, all Lua codes (not the runtime) are prepared by Users, so does envoy-apisix, so all Lua files in envoy-apisix need to be mounted into the istio-ingressgateway (gateway) and all istio-proxy (sidecar) containers.

We have a [configmap.go](configmap.go) script to iterate all Lua codes in envoy-apisix and create correpsonding configmap resouces.

We will run it twice for our demostrations.

The first run will generate necessary resources for the istio-ingressgateway, and the second one is for sidecars.

```sh
$ LUA_DIR=/path/to/envoy-apisix/lua go run configmap.go

Created configmap file configmaps/envoy-apisix-configmap-0
Created configmap file configmaps/envoy-apisix-configmap-1
Created configmap file configmaps/envoy-apisix-configmap-2
Created configmap file configmaps/envoy-apisix-configmap-3
Created kustomization.yaml

Run
	kubectl apply -k .

to install configmaps

Please add these flags when you use helm to install istiod/istio-ingressgateway

--set gateways.istio-ingressgateway.configVolumes\[0\].mountPath="/usr/local/share/Users/alex/Workstation/tokers/envoy-apisix/lua/apisix/core" \
--set gateways.istio-ingressgateway.configVolumes\[0\].name="envoy-apisix-configmap-0" \
--set gateways.istio-ingressgateway.configVolumes\[0\].configMapName="envoy-apisix-configmap-0" \
--set gateways.istio-ingressgateway.configVolumes\[1\].mountPath="/usr/local/share/Users/alex/Workstation/tokers/envoy-apisix/lua/apisix" \
--set gateways.istio-ingressgateway.configVolumes\[1\].name="envoy-apisix-configmap-1" \
--set gateways.istio-ingressgateway.configVolumes\[1\].configMapName="envoy-apisix-configmap-1" \
--set gateways.istio-ingressgateway.configVolumes\[2\].mountPath="/usr/local/share/Users/alex/Workstation/tokers/envoy-apisix/lua/apisix/plugins" \
--set gateways.istio-ingressgateway.configVolumes\[2\].name="envoy-apisix-configmap-2" \
--set gateways.istio-ingressgateway.configVolumes\[2\].configMapName="envoy-apisix-configmap-2" \
--set gateways.istio-ingressgateway.configVolumes\[3\].mountPath="/usr/local/share/Users/alex/Workstation/tokers/envoy-apisix/lua/deps/net" \
--set gateways.istio-ingressgateway.configVolumes\[3\].name="envoy-apisix-configmap-3" \
--set gateways.istio-ingressgateway.configVolumes\[3\].configMapName="envoy-apisix-configmap-3" \
```

Three kinds of resources are bumped:

1. a bunch of ConfigMap resources, each one contains several filename (base name) / codes key pairs.
2. a kustomization.yaml
3. a series of `--set` options, which guide you how to re-install istio-ingressgateway with these Lua codes.

So just use Kustomization to install these ConfigMap resources.

```sh
$ kubectl apply -k . -n istio-system
configmap/envoy-apisix-configmap-1 configured
configmap/envoy-apisix-configmap-2 configured
configmap/envoy-apisix-configmap-3 configured
```

As per the 3rd step, we should re-install the istio-ingressgateway, thanks for the good design of istio's helm charts, we don't need to hack it any more.

```sh
helm install -n istio-system istio-ingress manifests/charts/gateways/istio-ingress --set global.imagePullPolicy="IfNotPresent" --set global.hub="docker.io/istio" --set global.tag="1.8.1" --set global.jwtPolicy=first-party-jwt \
--set gateways.istio-ingressgateway.configVolumes\[0\].mountPath="/usr/local/share/lua/apisix/plugins" \
--set gateways.istio-ingressgateway.configVolumes\[0\].name="envoy-apisix-configmap-2" \
--set gateways.istio-ingressgateway.configVolumes\[0\].configMapName="envoy-apisix-configmap-2" \
--set gateways.istio-ingressgateway.configVolumes\[1\].mountPath="/usr/local/share/lua/deps/net" \
--set gateways.istio-ingressgateway.configVolumes\[1\].name="envoy-apisix-configmap-3" \
--set gateways.istio-ingressgateway.configVolumes\[1\].configMapName="envoy-apisix-configmap-3" \
--set gateways.istio-ingressgateway.configVolumes\[2\].mountPath="/usr/local/share/lua/apisix/core" \
--set gateways.istio-ingressgateway.configVolumes\[2\].name="envoy-apisix-configmap-0" \
--set gateways.istio-ingressgateway.configVolumes\[2\].configMapName="envoy-apisix-configmap-0" \
--set gateways.istio-ingressgateway.configVolumes\[3\].mountPath="/usr/local/share/lua/apisix" \
--set gateways.istio-ingressgateway.configVolumes\[3\].name="envoy-apisix-configmap-1" \
--set gateways.istio-ingressgateway.configVolumes\[3\].configMapName="envoy-apisix-configmap-1"
NAME: istio-ingress
LAST DEPLOYED: Tue Dec 22 21:47:57 2020
NAMESPACE: istio-system
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

And it's similar to change the istio-injector-template (to be continued).

Now try the sidecar.

```
$ NAMESPACE=bookinfo LUA_DIR=/path/to/envoy-apisix/lua go run configmap.go
Created configmap file configmaps/envoy-apisix-configmap-0
Created configmap file configmaps/envoy-apisix-configmap-1
Created configmap file configmaps/envoy-apisix-configmap-2
Created configmap file configmaps/envoy-apisix-configmap-3
Created kustomization.yaml

Run
	kubectl apply -k .

to install configmaps in namespace bookinfo

Please add the following annotations to your application Pod template

sidecar.istio.io/userVolume: |
   {"envoy-apisix-configmap-0":{"configMap":{"name":"envoy-apisix-configmap-0"},"name":"envoy-apisix-configmap-0"},"envoy-apisix-configmap-1":{"configMap":{"name":"envoy-apisix-configmap-1"},"name":"envoy-apisix-configmap-1"},"envoy-apisix-configmap-2":{"configMap":{"name":"envoy-apisix-configmap-2"},"name":"envoy-apisix-configmap-2"},"envoy-apisix-configmap-3":{"configMap":{"name":"envoy-apisix-configmap-3"},"name":"envoy-apisix-configmap-3"}}

sidecar.istio.io/userVolumeMount: |
   {"envoy-apisix-configmap-0":{"name":"envoy-apisix-configmap-0","mountPath":"/usr/local/share/lua/apisix/core"},"envoy-apisix-configmap-1":{"name":"envoy-apisix-configmap-1","mountPath":"/usr/local/share/lua/apisix"},"envoy-apisix-configmap-2":{"name":"envoy-apisix-configmap-2","mountPath":"/usr/local/share/lua/apisix/plugins"},"envoy-apisix-configmap-3":{"name":"envoy-apisix-configmap-3","mountPath":"/usr/local/share/lua/deps/net"}}
```

It also generates ConfigMap resources which need to be mounted to the istio-proxy container, Istio's injection template will check annotions `sidecar.istio.io/userVolume` and `sidecar.istio.io/userVolumeMount` in application pod template, which allows us to mount custom volumes.

Now edit the deployment of reviews, and scale it down and up.

```
$ kube edit -n bookinfo reviews-v1
# add annotations sidecar.istio.io/userVolumeMount and sidecar.istio.io/userVolume.

$ kube scale -n bookinfo --replicas 0
$ kube scale -n bookinfo --replicas 1
```

## Goal

We prepare to demostrate two plugins in envoy-apisix:

* Redirect
* URI Blocker

### Redirect

Redirect plugin intercepts the current request with configured status code and a custom Location header. We have [envoyfilter-ingressgateway.yaml](./samples/envoyfilter-ingressgateway.yaml) to let istio-ingressgateway returns 302 to https://apisix.apache.org when URI path is `/productpage`.

```sh
$ kubectl apply -f samples/envoyfilter-ingressgateway.yaml -n istio-system
```

Now try to access the istio-ingressgateway.

```sh
export ISTIO_INGRESSGATEWAY_PORT=$(kubectl get svc -n istio-system istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}')
export ISTIO_INGRESSGATEWAY_HOST=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}')

curl http://$ISTIO_INGRESSGATEWAY_HOST:$ISTIO_INGRESSGATEWAY_PORT/
productpage -vo /dev/null
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0*   Trying 192.168.64.3...
* TCP_NODELAY set
* Connected to 192.168.64.3 (192.168.64.3) port 30679 (#0)
> GET /productpage HTTP/1.1
> Host: 192.168.64.3:30679
> User-Agent: curl/7.64.1
> Accept: */*
>
< HTTP/1.1 302 Found
< location: https://apisix.apache.org
< server: istio-envoy
< content-length: 4
< date: Tue, 22 Dec 2020 10:16:45 GMT
```

When you see this output in your own machine, it means the plugin has already come into force.

### URI Blocker

URI Blocker allows you to deny some types of URI path and reject with custom status code. We have [envoyfilter-sidecar-reviews-v1.yaml](./samples/envoyfilter-sidecar-reviews-v1.yaml) to make the sidecar in reviews-v1 reject URI which contains `reviews`.

```
# first delete the legacy rules in istio-ingressgateway
$ kubectl delete -f samples/envoyfilter-ingressgateway.yaml -n istio-system
$ kubectl apply -f samples/envoyfilter-sidecar-reviews-v1.yaml -n bookinfo

export ISTIO_INGRESSGATEWAY_PORT=$(kubectl get svc -n istio-system istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}')
export ISTIO_INGRESSGATEWAY_HOST=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}')
```

Now open your browser and access `http://$ISTIO_INGRESSGATEWAY_HOST:$ISTIO_INGRESSGATEWAY_PORT/productpage`, refresh the page several times (be patient, since Envoy uses keep-alived connections), and you will see `Sorry, product reviews are currently unavailable for this book.` in the Bookinfo Reviews bar.
