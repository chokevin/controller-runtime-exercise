package controller

import (
	"context"
	"fmt"
	"os"

	appv1 "k8s.io/api/apps/v1"

	"github.com/steeling/controller-runtime-exercise/pkg/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type Controller struct {
	client  client.Client
	manager ctrl.Manager
}

func New(ctx context.Context) (*Controller, error) {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
	log := log.FromContext(ctx)
	log.Info("creating a new controller")
	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		return nil, err
	}

	if err := api.AddToScheme(manager.GetScheme()); err != nil {
		fmt.Printf("Unable to add the custom resource scheme: %v\n", err)
		os.Exit(1)
	}

	controller := &Controller{
		client:  manager.GetClient(),
		manager: manager,
	}

	err = ctrl.
		NewControllerManagedBy(manager). // Create the Controller
		For(&api.MyApp{}).               // MyApp is the Application API
		Owns(&appv1.Deployment{}).       // MyApp owns Deployments created by it
		Complete(controller)
	if err != nil {
		log.Error(err, "unable to create controller")
		return nil, err
	}
	return controller, nil
}

func (c *Controller) Start(ctx context.Context) error {
	return c.manager.Start(ctx)
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconcile request received", "name", req.Name, "namespace", req.Namespace)

	// Get the MyApp object for which the reconciliation is triggered
	myApp := &api.MyApp{}
	if err := c.client.Get(ctx, req.NamespacedName, myApp); err != nil {
		// Error handling
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists
	deployment := &appv1.Deployment{}
	dk := client.ObjectKey{
		Namespace: req.Namespace,
		Name:      myApp.Name,
	}
	err := c.client.Get(ctx, dk, deployment)

	if err != nil && client.IgnoreNotFound(err) == nil {
		// Create a new deployment
		deployment := &appv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: req.Namespace,
				Name:      myApp.Name,
			},
			Spec: appv1.DeploymentSpec{
				// Set the desired number of replicas
				Replicas: myApp.Spec.Replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: labelsForMyApp(myApp.Name),
				},
				// Set the template for the pods
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": myApp.Name,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  myApp.Name,
								Image: myApp.Spec.Image,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
								},
							},
						},
					},
				},
			},
		}

		if err := ctrl.SetControllerReference(myApp, deployment, c.manager.GetScheme()); err != nil {
			// Error handling
			return ctrl.Result{}, err
		}
		if err := c.client.Create(ctx, deployment); err != nil {
			// Error handling
			return ctrl.Result{}, err
		}
		// Deployment created successfully
		return ctrl.Result{}, nil
	}

	// Deployment already exists, do nothing
	return ctrl.Result{}, nil
}

// labelsForMyApp returns the labels for selecting the resources
// belonging to the given MyApp CR name.
func labelsForMyApp(name string) map[string]string {
	return map[string]string{"app": name}
}
