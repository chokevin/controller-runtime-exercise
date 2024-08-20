package controller

import (
	"context"
	"time"

	appv1 "k8s.io/api/apps/v1"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/steeling/controller-runtime-exercise/pkg/api"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// Define custom metrics
var (
	myAppReconcileCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "myapp_reconcile_total",
			Help: "Number of reconciliations for MyApp",
		},
		[]string{"namespace", "name"},
	)
	reconcileDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "myapp_reconcile_duration_seconds",
		Help:    "Duration of reconcile loop for MyApp",
		Buckets: prometheus.DefBuckets,
	}, []string{"result"})
)

const (
	reconcilationError   = "error"
	reconcilationSuccess = "success"
	reconcilationSkipped = "skipped"
)

type Controller struct {
	client  client.Client
	manager ctrl.Manager
}

func init() {
	metrics.Registry.MustRegister(myAppReconcileCounter, reconcileDuration)
}

func New(ctx context.Context) (*Controller, error) {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
	log := log.FromContext(ctx)
	log.Info("creating a new controller")
	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Metrics: metricsserver.Options{
			BindAddress: ":8080",
		},
		HealthProbeBindAddress: ":8081",
		LeaderElection:         false,
		LeaderElectionID:       "example-leader-election-id",
	})
	if err != nil {
		return nil, err
	}

	if err := manager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		return nil, err
	}

	if err := manager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		return nil, err
	}

	if err := api.AddToScheme(manager.GetScheme()); err != nil {
		log.Error(err, "Unable to add the custom resource scheme")
		return nil, err
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
	start := time.Now()
	log := log.FromContext(ctx)
	log.Info("reconcile request received", "name", req.Name, "namespace", req.Namespace)

	// Increment the custom metric counter
	myAppReconcileCounter.WithLabelValues(req.Namespace, req.Name).Inc()

	// Get the MyApp object for which the reconciliation is triggered
	myApp := &api.MyApp{}
	if err := c.client.Get(ctx, req.NamespacedName, myApp); err != nil {
		// Error handling
		reconcileDuration.WithLabelValues(reconcilationError).Observe(time.Since(start).Seconds())
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
		dp := createDeployment(myApp)

		if err := ctrl.SetControllerReference(myApp, dp, c.manager.GetScheme()); err != nil {
			// Error handling
			reconcileDuration.WithLabelValues(reconcilationError).Observe(time.Since(start).Seconds())
			return ctrl.Result{}, err
		}

		err := c.client.Create(ctx, dp)
		if err != nil {
			log.Error(err, "unable to create deployment")
			return ctrl.Result{}, err
		}

		reconcileDuration.WithLabelValues(reconcilationSuccess).Observe(time.Since(start).Seconds())
		return ctrl.Result{Requeue: true}, nil
	}

	// Check if PDB already exists
	pdb := &policyv1.PodDisruptionBudget{}
	pdbKey := client.ObjectKey{
		Namespace: req.Namespace,
		Name:      myApp.Name,
	}

	err = c.client.Get(ctx, pdbKey, pdb)
	if err != nil {
		// Create a new PDB
		pdb := createPodDisruptionBudget(myApp)

		if err := ctrl.SetControllerReference(myApp, pdb, c.manager.GetScheme()); err != nil {
			// Error handling
			reconcileDuration.WithLabelValues(reconcilationError).Observe(time.Since(start).Seconds())
			return ctrl.Result{}, err
		}

		err := c.client.Create(ctx, pdb)
		if err != nil {
			log.Error(err, "unable to create PDB")
			reconcileDuration.WithLabelValues(reconcilationError).Observe(time.Since(start).Seconds())
			return ctrl.Result{}, err
		}
		reconcileDuration.WithLabelValues(reconcilationSuccess).Observe(time.Since(start).Seconds())
		return ctrl.Result{Requeue: true}, nil
	}

	// Deployment already exists, do nothing
	reconcileDuration.WithLabelValues(reconcilationSkipped).Observe(time.Since(start).Seconds())
	return ctrl.Result{}, nil
}

// labelsForMyApp returns the labels for selecting the resources
// belonging to the given MyApp CR name.
func labelsForMyApp(name string) map[string]string {
	return map[string]string{"app": name}
}

func createDeployment(myApp *api.MyApp) *appv1.Deployment {
	deployment := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: myApp.Namespace,
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
							Args:  myApp.Spec.Args,
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
	return deployment
}

func createPodDisruptionBudget(myApp *api.MyApp) *policyv1.PodDisruptionBudget {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: myApp.Namespace,
			Name:      myApp.Name,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &intstr.IntOrString{
				IntVal: 1,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForMyApp(myApp.Name),
			},
		},
	}
	return pdb
}
