/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"

	orcav1beta1 "github.com/paolerm/orca-opcua-server/api/v1beta1"
)

// OpcuaServerReconciler reconciles a OpcuaServer object
type OpcuaServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const opcuaServerFinalizer = "paermini.com/opcua-finalizer"
const initialLBPort = 50000

//+kubebuilder:rbac:groups=orca.paermini.com,resources=opcuaservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=orca.paermini.com,resources=opcuaservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=orca.paermini.com,resources=opcuaservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=*,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpcuaServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *OpcuaServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	opcuaServer := &orcav1beta1.OpcuaServer{}
	err := r.Get(ctx, req.NamespacedName, opcuaServer)
	if err != nil {
		return ctrl.Result{}, err
	}

	isCrDeleted := opcuaServer.GetDeletionTimestamp() != nil
	if isCrDeleted {
		if controllerutil.ContainsFinalizer(opcuaServer, opcuaServerFinalizer) {
			// Run finalization logic. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeOpcuaServer(ctx, req, opcuaServer); err != nil {
				return ctrl.Result{}, err
			}

			// Remove opcuaServerTestFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(opcuaServer, opcuaServerFinalizer)
			err := r.Update(ctx, opcuaServer)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	opcuaNamePrefix := opcuaServer.Spec.Id
	numberOfServers := opcuaServer.Spec.ServerCount

	logger.Info("Getting all statefulSet under namespace " + req.NamespacedName.Namespace + " and assigned to simulation " + opcuaNamePrefix + "...")
	statefulSetList := &appsv1.StatefulSetList{}
	opts := []client.ListOption{
		client.InNamespace(req.NamespacedName.Namespace),
		client.MatchingLabels{"simulation": opcuaNamePrefix},
	}

	err = r.List(ctx, statefulSetList, opts...)
	if err != nil {
		logger.Error(err, "Failed to get statefulset list!")
		return ctrl.Result{}, err
	}

	existingStatefulSet := statefulSetList.Items
	for i := 0; i < numberOfServers; i++ {
		statefulSetName := opcuaNamePrefix + "-" + strconv.Itoa(i)

		statefulSet := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      statefulSetName,
				Namespace: req.NamespacedName.Namespace,
				Labels: map[string]string{
					"simulation": opcuaNamePrefix,
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"simulationLB": opcuaNamePrefix,
						"app":          statefulSetName,
					},
				},
				ServiceName: opcuaNamePrefix,
				Template: apiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"simulation":   opcuaNamePrefix,
							"simulationLB": opcuaNamePrefix,
							"app":          statefulSetName,
						},
						Annotations: map[string]string{
							"prometheus.io/path": "/metrics",
							"prometheus.io/port": "8080",
						},
					},
					Spec: apiv1.PodSpec{
						ImagePullSecrets: []apiv1.LocalObjectReference{
							{Name: "docker-secret"},
							{Name: "cdpx-acr-secret"},
						},
						Containers: []apiv1.Container{
							{
								Name:            opcuaNamePrefix,
								Image:           opcuaServer.Spec.DockerImageId,
								ImagePullPolicy: "Always",
								Args: []string{
									"--pn=50000",
									"--autoaccept",
									"--unsecuretransport",
									"--alm",
									"--ses",
									"--fastnodes=" + strconv.Itoa(opcuaServer.Spec.TagCount),
									"--veryfastrate=" + strconv.Itoa(opcuaServer.Spec.ChangeRateMs),
									"--fastnodesamplinginterval=" + strconv.Itoa(opcuaServer.Spec.SamplingIntervalMs),
									"--fasttype=uint",
									"--fasttyperandomization=True",
									"--ll=" + opcuaServer.Spec.LogLevel,             // set log level to debug, this level applies to opc plc code not opc ua stack
									"--llo=" + opcuaServer.Spec.OpcuaServerLogLevel, // set opc ua server log level to debug, this level applies to opc ua stack
									// "--la",        // TODO log to azure data explorer
								},
								Ports: []apiv1.ContainerPort{
									{
										Name:          "port-" + strconv.Itoa(i),
										ContainerPort: 50000,
									},
									{
										Name:          "http",
										ContainerPort: 8080,
										Protocol:      "TCP",
									},
								},
								// Env: []apiv1.EnvVar{}, TODO needed to implement ADX logs
							},
						},
					},
				},
			},
		}

		// Update VS Create
		contains, updatedList := containsAndRemove(existingStatefulSet, statefulSet)
		existingStatefulSet = updatedList
		if contains {
			logger.Info("Updating statefulSet under namespace " + statefulSet.ObjectMeta.Namespace + " with name " + statefulSet.ObjectMeta.Name + "...")
			err := r.Update(ctx, statefulSet)
			if err != nil {
				logger.Error(err, "Failed to update statefulSet under namespace "+statefulSet.ObjectMeta.Namespace+" with name "+statefulSet.ObjectMeta.Name)
				return ctrl.Result{}, err
			}
		} else {
			logger.Info("Creating statefulSet under namespace " + statefulSet.ObjectMeta.Namespace + " with name " + statefulSet.ObjectMeta.Name + "...")
			err := r.Create(ctx, statefulSet)
			if err != nil {
				logger.Error(err, "Failed to create statefulSet under namespace "+statefulSet.ObjectMeta.Namespace+" with name "+statefulSet.ObjectMeta.Name)
				return ctrl.Result{}, err
			}
		}
	}

	for i := 0; i < len(existingStatefulSet); i++ {
		logger.Info("Deleting statefulSet under namespace " + existingStatefulSet[i].ObjectMeta.Namespace + " with name " + existingStatefulSet[i].ObjectMeta.Name + "...")
		err := r.Delete(ctx, &existingStatefulSet[i])
		if err != nil {
			logger.Error(err, "Failed to delete statefulset!")
		}
	}

	// Load Balancer
	var ports []apiv1.ServicePort
	ports = make([]apiv1.ServicePort, numberOfServers)
	for i := 0; i < numberOfServers; i++ {
		ports[i] = apiv1.ServicePort{
			Name: opcuaNamePrefix + strconv.Itoa(i),
			Port: int32(initialLBPort + i),
			TargetPort: intstr.IntOrString{
				Type:   1,
				StrVal: "port-" + strconv.Itoa(i),
			},
		}
	}

	loadBalancer := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opcuaNamePrefix,
			Namespace: req.NamespacedName.Namespace,
			Labels: map[string]string{
				"simulation": opcuaNamePrefix,
			},
			Annotations: map[string]string{
				"service.beta.kubernetes.io/azure-load-balancer-internal": "true",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type:  "LoadBalancer",
			Ports: ports,
			Selector: map[string]string{
				"simulationLB": opcuaNamePrefix,
			},
		},
	}

	if opcuaServer.Spec.ServiceIp != "" {
		loadBalancer.Spec.LoadBalancerIP = opcuaServer.Spec.ServiceIp
	}

	logger.Info("Getting load balancer under namespace " + req.NamespacedName.Namespace + " and name " + opcuaNamePrefix + "...")
	existingLoadBalancer := &apiv1.Service{}
	lbName := types.NamespacedName{
		Name:      opcuaNamePrefix,
		Namespace: req.NamespacedName.Namespace,
	}
	err = r.Get(ctx, lbName, existingLoadBalancer)
	if err != nil {
		logger.Info("Creating load balancer under namespace " + req.NamespacedName.Namespace + " and name " + opcuaNamePrefix + "...")
		err := r.Create(ctx, loadBalancer)
		if err != nil {
			logger.Error(err, "Failed to create load balancer!")
			return ctrl.Result{}, err
		}
	} else {
		logger.Info("Updating load balancer under namespace " + req.NamespacedName.Namespace + " and name " + opcuaNamePrefix + "...")
		err := r.Update(ctx, loadBalancer)
		if err != nil {
			logger.Error(err, "Failed to update load balancer!")
			return ctrl.Result{}, err
		}
	}

	if loadBalancer.Status.LoadBalancer.Ingress != nil {
		ipAddress := make([]string, numberOfServers)
		for i := 0; i < numberOfServers; i++ {
			ipAddress[i] = loadBalancer.Status.LoadBalancer.Ingress[0].IP + ":" + strconv.Itoa(initialLBPort+i)
		}

		logger.Info("Setting all publicIpAddress to into CR.")
		opcuaServer.Status.PublicIpAddress = ipAddress

		err = r.Status().Update(ctx, opcuaServer)
		if err != nil {
			logger.Error(err, "Failed to update CR!")
			return ctrl.Result{}, err
		}
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(opcuaServer, opcuaServerFinalizer) {
		controllerutil.AddFinalizer(opcuaServer, opcuaServerFinalizer)
		err = r.Update(ctx, opcuaServer)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{Requeue: true}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpcuaServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&orcav1beta1.OpcuaServer{}).
		Complete(r)
}

func (r *OpcuaServerReconciler) finalizeOpcuaServer(ctx context.Context, req ctrl.Request, opcuaServer *orcav1beta1.OpcuaServer) error {
	logger := log.FromContext(ctx)
	opcuaServerNamePrefix := opcuaServer.Spec.Id

	statefulSetList := &appsv1.StatefulSetList{}
	opts := []client.ListOption{
		client.InNamespace(req.NamespacedName.Namespace),
		client.MatchingLabels{"simulation": opcuaServerNamePrefix},
	}

	err := r.List(ctx, statefulSetList, opts...)
	if err != nil {
		// TODO: ignore not found error
		logger.Error(err, "Failed to get statefulset list!")
		return err
	}

	logger.Info("CR with name " + req.NamespacedName.Name + " under namespace " + req.NamespacedName.Namespace + " marked as deleted. Deleting statefulSet...")
	for i := 0; i < len(statefulSetList.Items); i++ {
		err = r.Delete(ctx, &statefulSetList.Items[i])
		if err != nil {
			logger.Error(err, "Failed to delete statefulset!")
			return err
		}
	}

	logger.Info("Getting load balancer under namespace " + opcuaServer.ObjectMeta.Namespace + " and name " + opcuaServerNamePrefix + "...")

	loadBalancer := &apiv1.Service{}
	lbName := types.NamespacedName{
		Name:      opcuaServerNamePrefix,
		Namespace: opcuaServer.ObjectMeta.Namespace,
	}

	err = r.Get(ctx, lbName, loadBalancer)
	if err != nil {
		// TODO: ignore not found error
		logger.Error(err, "Failed to get load balancer!")
		return err
	}

	logger.Info("Removing load balancer under namespace " + req.NamespacedName.Namespace + " and name " + opcuaServerNamePrefix + "...")
	err = r.Delete(ctx, loadBalancer)
	if err != nil {
		logger.Error(err, "Failed to delete load balancer!")
		return err
	}

	logger.Info("Successfully finalized")
	return nil
}

func containsAndRemove(s []appsv1.StatefulSet, e *appsv1.StatefulSet) (bool, []appsv1.StatefulSet) {
	for i, a := range s {
		if a.ObjectMeta.Namespace == e.ObjectMeta.Namespace && a.ObjectMeta.Name == e.ObjectMeta.Name {
			return true, append(s[:i], s[i+1:]...)
		}
	}
	return false, s
}
