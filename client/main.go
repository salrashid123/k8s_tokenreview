package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat/go-jwx/jwk"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	// "k8s.io/client-go/rest"
	// //serviceaccount "k8s.io/kubernetes/pkg/serviceaccount"
)

var (
	jwtSet  *jwk.Set
	jwkfile = flag.String("jwk", "jwk.json", "JWK File")
	token   = flag.String("token", "eyJhbGciOiJSUzI1NiIsImtpZCI6InBsNDY3VmlzaG1nekFYVldULW92LVU4SHRYVGNHS2hBcHJQRF9tbUQyLWcifQ.eyJhdWQiOlsiZ2NwLXN0cy1hdWRpZW5jZSJdLCJleHAiOjE2ODg1OTQ3NzIsImlhdCI6MTY4ODU4NzU3MiwiaXNzIjoiaHR0cHM6Ly8wYjczLTI2MDAtNDA0MC0yMDk4LWE3MDAtZDQyNy05MGFlLWY2ZDEtMmRkZC5uZ3Jvay5pbyIsImt1YmVybmV0ZXMuaW8iOnsibmFtZXNwYWNlIjoiZGVmYXVsdCIsInBvZCI6eyJuYW1lIjoibXlhcHAtZGVwbG95bWVudC1jNjY3OTk0Y2QtNHptd3oiLCJ1aWQiOiIyNTIxYzE5ZS05MTVkLTQ3M2MtOTNhMy1hOTk3MDc5MDUxNTMifSwic2VydmljZWFjY291bnQiOnsibmFtZSI6InN2YzEtc2EiLCJ1aWQiOiI5ZGQ5NDM4Ny01ODNmLTRkYjItYmFjMC01OTE5MGY4YzBhNzgifX0sIm5iZiI6MTY4ODU4NzU3Miwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmRlZmF1bHQ6c3ZjMS1zYSJ9.Jk4l0fWsLD01RYng7aY_D8G6OsQ5zfvGHpF2BCg6tcDCR2ulU2iSsX4prLwkAyLY7bfmigwDzVG5stpEka0cCSAlQ2XrUJ_stZhoy2zJbHp12T3uigY3orUExmQSd2-4DGojxhfInfBJqnFljjrzZ6XhcR6olcZNr2MsZxAWSdCPhugH5rB-ji-2a6B9IO7q72O4E9YFJPYkxkU7M-Q_WExKZp5YUTQ9caZtebEhmNztxxsK8kJsWGAafvmGHe0hQeyjeJ1YfPmlIawqCHyylDikCk5MKtB34ibq2poEdyOHHNStXx0rv_NORfLGq-ZR75av90igB76ZWsYHPaKaWw", "ServiceAccount JWT Token to validate")
)

const ()

type serviceAccount struct {
	Name       string `json:"kubernetes.io/serviceaccount/service-account.name"`
	UID        string `json:"kubernetes.io/serviceaccount/service-account.uid"`
	SecretName string `json:"kubernetes.io/serviceaccount/secret.name"`
	Namespace  string `json:"kubernetes.io/serviceaccount/namespace"`

	// the JSON returned from reviewing a Projected Service account has a
	// different structure, where the information is in a sub-structure instead of
	// at the top level
	Kubernetes *projectedServiceToken `json:"kubernetes.io"`
	Expiration int64                  `json:"exp"`
	IssuedAt   int64                  `json:"iat"`
	jwt.RegisteredClaims
}

type projectedServiceToken struct {
	Namespace      string        `json:"namespace"`
	Pod            *k8sObjectRef `json:"pod"`
	ServiceAccount *k8sObjectRef `json:"serviceaccount"`
}

type k8sObjectRef struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

func getKey(token *jwt.Token) (interface{}, error) {
	keyID, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errors.New("expecting JWT header to have string kid")
	}
	if key := jwtSet.LookupKeyID(keyID); len(key) == 1 {
		return key[0].Materialize()
	}
	return nil, errors.New("unable to find key")
}

func verifyClusterIDToken(ctx context.Context, rawToken string) (serviceAccount, error) {
	token, err := jwt.ParseWithClaims(rawToken, &serviceAccount{}, getKey)
	if err != nil {
		fmt.Printf("     Error parsing JWT %v", err)
		return serviceAccount{}, err
	}
	if claims, ok := token.Claims.(*serviceAccount); ok && token.Valid {
		return *claims, nil
	}
	return serviceAccount{}, errors.New("error parsing JWT Claims")
}

func main() {
	flag.Parse()

	ctx := context.Background()

	// Using JWT Validation
	fmt.Println("Using JWT Validation")

	var err error
	j, err := os.ReadFile(*jwkfile)
	if err != nil {
		fmt.Printf("Unable to read jwk file: %v", err)
		return
	}
	jwtSet, err = jwk.Parse(j)
	if err != nil {
		fmt.Printf("Unable to load JWK Set: %v", err)
		return
	}
	doc, err := verifyClusterIDToken(ctx, *token)
	if err != nil {
		fmt.Printf("Unable to verify IDTOKEN: %v", err)
		return
	}
	fmt.Printf("OIDC signature verified  with Audience [%s] Issuer [%s] and PodName [%s]\n", doc.Audience, doc.RegisteredClaims.Issuer, doc.Kubernetes.Pod.UID)

	// Using TokenReview API

	fmt.Println("Using TokenReview API")

	// if your local config allows you to access the k8s api for tokenreview
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("error getting user home dir: %v\n", err)
		os.Exit(1)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		fmt.Printf("Error getting kubernetes config: %v\n", err)
		os.Exit(1)
	}

	// if you are running somwhere inside the clsuter and you have the rbac role to use the review
	// kubeConfig, err := rest.InClusterConfig()
	// if err != nil {
	// 	fmt.Printf("Error getting kubernetes config: %v\n", err)
	// 	os.Exit(1)
	// }

	// or just access remotely anon
	// kubeConfig := rest.AnonymousClientConfig(&rest.Config{
	// 	Host: *host,
	// })

	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		fmt.Printf("error getting kubernetes config: %v\n", err)
	}

	tr := &authenticationv1.TokenReview{
		Spec: authenticationv1.TokenReviewSpec{
			Token:     *token,
			Audiences: []string{"gcp-sts-audience"},
		},
	}
	result, err := client.AuthenticationV1().TokenReviews().Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("error getting kubernetes config: %v\n", err)
		return
	}
	fmt.Printf("TokenReview Verified with user UID [%s];  Authenticated: %t\n", result.Status.User.UID, result.Status.Authenticated)
}
