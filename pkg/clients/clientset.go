package clients

import (
	"flag"

	chaosClient "github.com/litmuschaos/chaos-operator/pkg/client/clientset/versioned/typed/litmuschaos/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientSets is a collection of clientSets and kubeConfig needed
type ClientSets struct {
	KubeClient    *kubernetes.Clientset
	LitmusClient  *chaosClient.LitmuschaosV1alpha1Client
	KubeConfig    *rest.Config
	DynamicClient dynamic.Interface
}

// GenerateClientSetFromKubeConfig will generation both ClientSets (k8s, and Litmus) as well as the KubeConfig
func (c *ClientSets) GenerateClientSetFromKubeConfig() error {

	if err := c.getKubeConfig(); err != nil {
		return err
	}
	if err := c.generateK8sClientSet(); err != nil {
		return err
	}
	if err := c.generateLitmusClientSet(); err != nil {
		return err
	}
	if err := c.generateDynamicClientSet(); err != nil {
		return err
	}
	return nil
}

// GetKubeConfig setup the config for access cluster resource
func (c *ClientSets) getKubeConfig() error {
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Parse()
	// It uses in-cluster config, if kubeconfig path is not specified
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return err
	}
	c.KubeConfig = config
	return nil
}

// generateK8sClientSet will generation k8s client
func (c *ClientSets) generateK8sClientSet() error {
	k8sClientSet, err := kubernetes.NewForConfig(c.KubeConfig)
	if err != nil {
		return errors.Wrapf(err, "Unable to generate kubernetes clientSet, err: %v: ", err)
	}
	c.KubeClient = k8sClientSet
	return nil
}

// generateLitmusClientSet will generate a LitmusClient
func (c *ClientSets) generateLitmusClientSet() error {
	litmusClientSet, err := chaosClient.NewForConfig(c.KubeConfig)
	if err != nil {
		return errors.Wrapf(err, "Unable to create LitmusClientSet, err: %v", err)
	}
	c.LitmusClient = litmusClientSet
	return nil
}

// generateDynamicClientSet will generate a DynamicClient
func (c *ClientSets) generateDynamicClientSet() error {
	dynamicClientSet, err := dynamic.NewForConfig(c.KubeConfig)
	if err != nil {
		return errors.Wrapf(err, "Unable to create DynamicClientSet, err: %v", err)
	}
	c.DynamicClient = dynamicClientSet
	return nil
}
