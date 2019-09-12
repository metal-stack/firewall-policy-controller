package main

import (
	"path"
	"testing"

	"io/ioutil"
	"log"

	"github.com/ghodss/yaml"
	assert "github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestPolicyEnforcementWithTestData(t *testing.T) {
	for _, tc := range list("test_data", true) {
		t.Run(tc, func(t *testing.T) {
			tcd := path.Join("test_data", tc)
			c := testclient.NewSimpleClientset()
			npl := &networkingv1.NetworkPolicyList{}
			for _, i := range list(path.Join(tcd, "services"), false) {
				var svc corev1.Service
				mustUnmarshal(path.Join(tcd, "services", i), &svc)
				c.CoreV1().Services(svc.ObjectMeta.Namespace).Create(&svc)
			}
			for _, i := range list(path.Join(tcd, "policies"), false) {
				var np networkingv1.NetworkPolicy
				mustUnmarshal(path.Join(tcd, "policies", i), &np)
				npl.Items = append(npl.Items, np)
			}
			fw := NewMetalFirewall(c)
			fw.AssembleRules(npl)
			exp, _ := ioutil.ReadFile(path.Join(tcd, "expected.nftablev4"))
			assert.Equal(t, string(exp), fw.render())
		})
	}
}

func list(path string, dirs bool) []string {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	r := []string{}
	for _, f := range files {
		if f.IsDir() && dirs {
			r = append(r, f.Name())
		} else if !f.IsDir() && !dirs {
			r = append(r, f.Name())
		}
	}
	return r
}

func mustUnmarshal(f string, data interface{}) {
	c, _ := ioutil.ReadFile(f)
	err := yaml.Unmarshal(c, data)
	if err != nil {
		panic(err)
	}
}
