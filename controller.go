package main

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	keyvaultsecretv1alpha1 "github.com/twendt/secret-controller/pkg/apis/secretcontroller/v1alpha1"
	clientset "github.com/twendt/secret-controller/pkg/client/clientset/versioned"
	secretscheme "github.com/twendt/secret-controller/pkg/client/clientset/versioned/scheme"
	informers "github.com/twendt/secret-controller/pkg/client/informers/externalversions/secretcontroller/v1alpha1"
	listers "github.com/twendt/secret-controller/pkg/client/listers/secretcontroller/v1alpha1"
	"github.com/twendt/secret-controller/pkg/secretstore"
)

const controllerAgentName = "secret-controller"

const (
	// SecretCreated is used as part of the Event 'reason' when a Foo is synced
	SecretCreated = "Created"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageSecretCreated is the message used for an Event fired when a KeyvaultSecret
	// is created successfully
	MessageSecretCreated = "Key Vault Secret created successfully"
)

// Controller is the controller implementation for KeyvaultSecret resources
type Controller struct {
	kubeclientset          kubernetes.Interface
	crdclientset           clientset.Interface
	keyvaultClient         secretstore.Client
	keyvaultSecretInformer informers.KeyvaultSecretInformer
	keyvaultSecretsLister  listers.KeyvaultSecretLister
	keyvaultSecretsSynced  cache.InformerSynced
	secretsLister          corelisters.SecretLister
	workqueue              workqueue.RateLimitingInterface
	recorder               record.EventRecorder
	logger                 *logrus.Entry
}

// NewController returns a new controller
func NewController(
	kubeclientset kubernetes.Interface,
	crdclientset clientset.Interface,
	kubeInformer coreinformers.SecretInformer,
	keyvaultSecretInformer informers.KeyvaultSecretInformer,
	keyvaultClient secretstore.Client,
	logger *logrus.Entry) *Controller {

	utilruntime.Must(secretscheme.AddToScheme(scheme.Scheme))

	logger.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logger.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:          kubeclientset,
		crdclientset:           crdclientset,
		keyvaultClient:         keyvaultClient,
		keyvaultSecretInformer: keyvaultSecretInformer,
		keyvaultSecretsLister:  keyvaultSecretInformer.Lister(),
		keyvaultSecretsSynced:  keyvaultSecretInformer.Informer().HasSynced,
		secretsLister:          kubeInformer.Lister(),
		workqueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "KeyvaultSecrets"),
		recorder:               recorder,
		logger:                 logger,
	}

	return controller
}

// Run starts the worker go routines
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	c.logger.Info("Starting Secret controller")
	c.setupWatches()

	c.logger.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.keyvaultSecretsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	c.logger.Info("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	c.logger.Info("Started workers")
	<-stopCh
	c.logger.Info("Shutting down workers")

	return nil
}

func (c *Controller) setupWatches() {
	c.logger.Info("Setting up event handlers")
	c.keyvaultSecretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			var key string
			var err error
			if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
				runtime.HandleError(err)
				return
			}
			c.workqueue.AddRateLimited(key)
		},
		UpdateFunc: func(old, new interface{}) {
			oldObj := old.(*keyvaultsecretv1alpha1.KeyvaultSecret)
			newObj := new.(*keyvaultsecretv1alpha1.KeyvaultSecret)
			if oldObj.GetResourceVersion() != newObj.GetResourceVersion() {
				c.enqueueKeyvaultSecret(new)
			}
		},
		DeleteFunc: func(obj interface{}) {
			var key string
			var err error
			if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
				runtime.HandleError(err)
				return
			}
			c.workqueue.AddRateLimited(key)
		},
	})
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := c.processItem(obj)
	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processItem(obj interface{}) error {
	defer c.workqueue.Done(obj)
	var key string
	var ok bool
	if key, ok = obj.(string); !ok {
		c.workqueue.Forget(obj)
		runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
		return nil
	}
	if err := c.secretHandler(key); err != nil {
		c.workqueue.AddRateLimited(key)
		return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
	}
	c.workqueue.Forget(obj)
	c.logger.Infof("Successfully synced '%s'", key)
	return nil
}

func (c *Controller) secretHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, secretName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	keyvaultSecret, err := c.keyvaultSecretsLister.KeyvaultSecrets(namespace).Get(secretName)
	if errors.IsNotFound(err) {
		c.logger.Infof("keyvault secret key '%s' deleted", key)
		return nil
	}
	if err != nil {
		return err
	}

	if err := c.createOrUpdateSecret(keyvaultSecret); err != nil {
		return err
	}
	c.recorder.Event(keyvaultSecret, corev1.EventTypeNormal, SecretCreated, MessageSecretCreated)
	return nil
}

func (c *Controller) createOrUpdateSecret(keyvaultSecret *keyvaultsecretv1alpha1.KeyvaultSecret) error {
	converter := SecretConverter{keyvaultSecret: keyvaultSecret, storeClient: c.keyvaultClient}
	secret, err := converter.getK8sSecret()
	if err != nil {
		return err
	}
	_, err = c.kubeclientset.CoreV1().Secrets(keyvaultSecret.Namespace).Update(secret)
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = c.kubeclientset.CoreV1().Secrets(keyvaultSecret.Namespace).Create(secret)
			if err != nil {
				c.logger.Errorf("Failed to create secret %s : %s", secret.Name, err)
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

func (c *Controller) enqueueKeyvaultSecret(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}
