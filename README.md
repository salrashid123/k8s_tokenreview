# Kubernetes TokenReview API with Minikube

Simple demo of using [Kubernetes TokenReview API](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-review-v1/) to validate a k8s service account JWT bearer token.

This repo is nothing new other than a way to demonstrate how you can validate an k8s sa token using both a standard `JWT Validator` and the `TokenReview` API

The JWT validator simply verifies if a token is signed correctly and has some claims.  The signature validation is done by retrieving the JWKs certs directly from the target kubernetes API server endpoit.

The TokenReview is done by invoking the api endpoint directly

--this repo is nothing new; just wrote it here for my ref--

![images/flow.png](images/flow.png)

(image taken from [vmware article](https://tanzu.vmware.com/developer/guides/platform-security-workload-identity/))

---


Other references

- [Kubernetes WebHook Authentication/Authorization Minikube HelloWorld](https://github.com/salrashid123/k8s_webhook_helloworld)
- [Using kubernetes TokenReviews go api on pod](https://gist.github.com/salrashid123/75c22afcbdbf1b706ab76d9063122429)
- [Using Kubernetes Service Accounts for Google Workload Identity Federation](https://github.com/salrashid123/k8s_federation_with_gcp)


### Setup

First install the following on your laptop

* [minikube](https://minikube.sigs.k8s.io/docs/)
* [ngrok](https://ngrok.com/)
* optionally [jq](https://stedolan.github.io/jq/)


### Configure ngrok Tunnel

First Step is to run `ngrok` and find out the URL its assigned to you.

The only reason why w'ere usign ngrok is to give a public ip address; you don't have to, but its just easier this way for the demo

```bash
./ngrok http -host-header=rewrite  localhost:8080

## you'll get a publicAddress, just note it down
export DISCOVERY_URL="https://e955-2600-4040-2098-a700-c12-d391-3ae8-35dd.ngrok.io"
```

Now start minikube and enable the jwk url that points to the server

```bash
minikube stop
minikube delete

minikube start --driver=kvm2 --embed-certs \
    --extra-config=apiserver.service-account-jwks-uri=$DISCOVERY_URL/openid/v1/jwks \
    --extra-config=apiserver.service-account-issuer=$DISCOVERY_URL   

# new window, create a proxy back
kubectl proxy --port=8080  --accept-paths="^/\.well-known\/openid-configuration|^/openid\/v1\/jwks|^\/apis\/authentication.k8s.io\/v1\/tokenreviews" 

kubectl create clusterrolebinding oidc-reviewer --clusterrole=system:service-account-issuer-discovery --group=system:unauthenticated

# test that you can see the jwks endpoint/oidc config
curl -s $DISCOVERY_URL/.well-known/openid-configuration | jq '.'

kubectl get --raw /.well-known/openid-configuration | jq -r .issuer

## apply a sample deployment which mounts a svc account token to /var/run/secrets/iot-token
kubectl apply -f my-deployment.yaml
```

In my case, i saw

```bash
$ kubectl get po
NAME                               READY   STATUS    RESTARTS   AGE
myapp-deployment-c667994cd-4klvs   1/1     Running   0          24m
myapp-deployment-c667994cd-5mhsn   1/1     Running   0          24m

$ export SA_TOKEN=`kubectl exec myapp-deployment-c667994cd-4klvs -- cat /var/run/secrets/iot-token/iot-token`

$ echo $SA_TOKEN
eyJhbGciOiJSUzI1NiIsImtpZCI6Ik9lRW9Sanh1RzVzMllvT2liSVZIemZzZi10Zm5lM3Rwdm5UcmNDbDNHeTgifQ.eyJhdWQiOlsiZ2NwLXN0cy1hdWRpZW5jZSJdLCJleHAiOjE2ODg3NTQ5MzgsImlhdCI6MTY4ODc0NzczOCwiaXNzIjoiaHR0cHM6Ly9lOTU1LTI2MDAtNDA0MC0yMDk4LWE3MDAtYzEyLWQzOTEtM2FlOC0zNWRkLm5ncm9rLmlvIiwia3ViZXJuZXRlcy5pbyI6eyJuYW1lc3BhY2UiOiJkZWZhdWx0IiwicG9kIjp7Im5hbWUiOiJteWFwcC1kZXBsb3ltZW50LWM2Njc5OTRjZC00a2x2cyIsInVpZCI6IjBjNGU1ZDc3LTJmY2ItNDY5My04NmMzLWZlMzlmZWUzOWM0YSJ9LCJzZXJ2aWNlYWNjb3VudCI6eyJuYW1lIjoic3ZjMS1zYSIsInVpZCI6IjhiMDhkOTM4LTg3MzAtNGU1Ni05ZDkzLThiZTMyZjgyMDI4NyJ9fSwibmJmIjoxNjg4NzQ3NzM4LCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6ZGVmYXVsdDpzdmMxLXNhIn0.AG2Cn1sMYEdh_ezbjsU8HFYo-OzLjbXe0HwtomIlE0s7dfJf8gxcy31UFL-jpdqkyoGBs1YnqcbZWCD36_Sq_6HWNYqXgd_SaeAwA7QT1jS0_rbN99jYMfFtddNCSsFSpSdM6RQ02qgTrSMGgQ0zOTtPZf3KNY-L7kS7y0OTKnzw6632EysU0q7gI-qBUbTH2U6n7r9eWRhpDdgJdH3P7Efx0OlmmQigCwgwExWMfPsvOzdQo13i2--N1nTI_evrhF-41YLr_v_xEBLx67AH6weZVpM1eQtE3gSRNqDlcbR65_N69CaTW8zUF5LHmy39XbqSUnap4Kles8jHWvju6A
```

Note the JWT there is in the form

```json
{
  "alg": "RS256",
  "kid": "OeEoRjxuG5s2YoOibIVHzfsf-tfne3tpvnTrcCl3Gy8"
}.
{
  "aud": [
    "gcp-sts-audience"
  ],
  "exp": 1688754938,
  "iat": 1688747738,
  "iss": "https://e955-2600-4040-2098-a700-c12-d391-3ae8-35dd.ngrok.io",
  "kubernetes.io": {
    "namespace": "default",
    "pod": {
      "name": "myapp-deployment-c667994cd-4klvs",
      "uid": "0c4e5d77-2fcb-4693-86c3-fe39fee39c4a"
    },
    "serviceaccount": {
      "name": "svc1-sa",
      "uid": "8b08d938-8730-4e56-9d93-8be32f820287"
    }
  },
  "nbf": 1688747738,
  "sub": "system:serviceaccount:default:svc1-sa"
}
```


to validate it two ways:

```bash
$ go run main.go --host=$DISCOVERY_URL --token=$SA_TOKEN

Using JWT Validation
JWKS URL https://0c71-2600-4040-2098-a700-c12-d391-3ae8-35dd.ngrok.io/openid/v1/jwks
OIDC signature verified  with Audience [[gcp-sts-audience]] Issuer [https://e955-2600-4040-2098-a700-c12-d391-3ae8-35dd.ngrok.io] and PodName [0c4e5d77-2fcb-4693-86c3-fe39fee39c4a]
Using TokenReview API
TokenReview Verified with user UID [8b08d938-8730-4e56-9d93-8be32f820287];  Authenticated: true
```


