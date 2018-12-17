package main

import (
	"flag"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	clientset "github.com/twendt/secret-controller/pkg/client/clientset/versioned"
	informers "github.com/twendt/secret-controller/pkg/client/informers/externalversions"
	"github.com/twendt/secret-controller/pkg/secretstore/keyvault"
	"github.com/twendt/secret-controller/pkg/signals"
)

var (
	masterURL  string
	kubeconfig string
	vaultName  string
)

func main() {
	logrus.SetOutput(os.Stdout)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetReportCaller(true)

	flag.Parse()

	logger := logrus.WithFields(logrus.Fields{
		"vaultName": vaultName,
	})

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		logrus.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logrus.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	crdClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		logrus.Fatalf("Error building example clientset: %s", err.Error())
	}

	keyvaultClient, err := keyvault.NewVaultClient(vaultName)
	if err != nil {
		logrus.Fatalln("Could not get Key Vault Client:", err)
	}

	crdInformerFactory := informers.NewSharedInformerFactory(crdClient, time.Second*30)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)

	controller := NewController(kubeClient, crdClient,
		kubeInformerFactory.Core().V1().Secrets(),
		crdInformerFactory.Secretcontroller().V1alpha1().KeyvaultSecrets(),
		keyvaultClient,
		logger)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	crdInformerFactory.Start(stopCh)

	if err = controller.Run(1, stopCh); err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&vaultName, "vault-name", "", "Name of Azure Key Vault to use")
}
