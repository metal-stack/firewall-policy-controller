package main

import (
	"fmt"
	"io/ioutil"

	"github.com/ghodss/yaml"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// c, err := loadClient(".kubeconfig")
	// if err != nil {
	// 	panic(err)
	// }
	// npl, err := c.NetworkingV1().NetworkPolicies("default").List(metav1.ListOptions{})
	// if err != nil {
	// 	panic(err)
	// }
	// fw := NewMetalFirewall(c)
	// err = fw.AssembleRules(npl)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Sprint(fw.render())
}

func loadClient(kubeconfigPath string) (*k8s.Clientset, error) {
	data, err := ioutil.ReadFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("read kubeconfig: %v", err)
	}
	var config rest.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal kubeconfig: %v", err)
	}
	return k8s.NewForConfig(&config)
}
