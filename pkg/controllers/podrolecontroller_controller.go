/*
Copyright 2021.

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
	"fmt"
	"reflect"

	utilities "github.com/kaladaopuiyo/pod-role-controller/pkg/utilities"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	v1alpha1 "github.com/kaladaopuiyo/pod-role-controller/api/v1alpha1"
)

// PodRoleControllerReconciler reconciles a PodRoleController object
type PodRoleControllerReconciler struct {
	client.Client
	KubeClient kubernetes.Interface
	Log        logr.Logger
	Scheme     *runtime.Scheme
}

var (
	err               error
	podRoleController = &v1alpha1.PodRoleController{}
)

//+kubebuilder:rbac:groups=controller.pod.role.controller.io,resources=podrolecontrollers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controller.pod.role.controller.io,resources=podrolecontrollers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=controller.pod.role.controller.io,resources=podrolecontrollers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;
//+kubebuilder:rbac:groups="",resources=pods/exec;pods/logs;pods/portforward,verbs=get;list;watch;
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PodRoleController object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *PodRoleControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	r.Log = log.FromContext(ctx)
	r.Log.WithValues("pod-role-controller", req.NamespacedName)

	replicaSet := &appsv1.ReplicaSet{}
	role := &rbacv1.Role{}

	err = r.Client.Get(ctx, req.NamespacedName, podRoleController)
	reconcilePodRoleController := true

	if err != nil {

		err = r.Client.Get(ctx, req.NamespacedName, role)
		reconcileRole := true

		if err != nil {

			err = r.Client.Get(ctx, req.NamespacedName, replicaSet)
			reconcileReplicaSet := true

			if err != nil {
				if errors.IsNotFound(err) {
					// Request object not found, could have been deleted after reconcile request.
					// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
					// Return and don't requeue
					return reconcile.Result{}, nil
				}
				return reconcile.Result{}, err
			}

			if reconcileReplicaSet {

				// check if labelSelector for pod matches any of the deployments
				index, deploymentFound := utilities.Find(podRoleController.Spec.Deployments, replicaSet.Labels[podRoleController.Spec.LabelSelector])

				if deploymentFound && req.Namespace == podRoleController.Namespace && replicaSet.Status.Replicas != int32(0) {
					r.Log.Info("Reconcile ReplicaSet changes to deployment " + podRoleController.Spec.Deployments[index] + " in namespace " + req.Namespace)

					role, err = r.KubeClient.RbacV1().Roles(req.Namespace).Get(ctx, podRoleController.Spec.Role, metav1.GetOptions{})
					if err != nil {
						if errors.IsNotFound(err) {
							// Request object not found, could have been deleted after reconcile request.
							// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
							// Return and don't requeue
							r.Log.Info("Role " + podRoleController.Spec.Role + " in namespace " + req.Namespace + " was not found")
							return reconcile.Result{}, client.IgnoreNotFound(err)
						}
						return reconcile.Result{}, err
					}

					copy := role.DeepCopy()
					original := role.DeepCopy()

					rules, err := r.updateRules(ctx, original, podRoleController.Spec.Deployments[index], req.NamespacedName)
					if err != nil {
						return ctrl.Result{}, err
					}

					copy.Rules = rules

					err = r.updateRole(ctx, copy, original, req.Name)
					if err != nil {
						return ctrl.Result{}, err
					}

					r.Log.Info("Reconciliation completed for deployment: " + podRoleController.Spec.Deployments[index] + " in namespace " + req.Namespace)
				}

				reconcileRole = false

			}

		}

		if reconcileRole {

			namespacedName := types.NamespacedName{Name: podRoleController.Spec.Role, Namespace: podRoleController.Namespace}

			if req.NamespacedName == namespacedName {

				r.Log.Info("Reconcile changes to role " + podRoleController.Spec.Role + " in namespace " + req.Namespace)

				for _, deploy := range podRoleController.Spec.Deployments {

					role, err = r.KubeClient.RbacV1().Roles(req.Namespace).Get(ctx, podRoleController.Spec.Role, metav1.GetOptions{})
					if err != nil {
						if errors.IsNotFound(err) {
							// Request object not found, could have been deleted after reconcile request.
							// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
							// Return and don't requeue
							r.Log.Info("Role " + podRoleController.Spec.Role + " in namespace " + req.Namespace + " was not found")
							return reconcile.Result{}, client.IgnoreNotFound(err)
						}
						return reconcile.Result{}, err
					}

					copy := role.DeepCopy()
					original := role.DeepCopy()

					rules, err := r.updateRules(ctx, original, deploy, req.NamespacedName)
					if err != nil {
						return ctrl.Result{}, err
					}

					copy.Rules = rules

					err = r.updateRole(ctx, copy, original, podRoleController.Spec.Role)
					if err != nil {
						return ctrl.Result{}, err
					}

				}

				r.Log.Info("Reconciliation completed for role: " + podRoleController.Spec.Role + " in namespace " + podRoleController.Namespace)
			}
		}
		reconcilePodRoleController = false

	}

	if reconcilePodRoleController {

		namespacedName := types.NamespacedName{Name: podRoleController.Name, Namespace: podRoleController.Namespace}

		if req.NamespacedName == namespacedName {

			r.Log.Info("Reconcile  podRoleController changes for " + req.Name + " in namespace " + req.Namespace)
			role, err = r.KubeClient.RbacV1().Roles(req.Namespace).Get(ctx, podRoleController.Spec.Role, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					// Request object not found, could have been deleted after reconcile request.
					// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
					// Return and don't requeue
					r.Log.Info("Role " + podRoleController.Spec.Role + " in namespace " + req.Namespace + " was not found")
					return reconcile.Result{}, client.IgnoreNotFound(err)
				}
				return reconcile.Result{}, err
			}

			rules := []rbacv1.PolicyRule{}
			copy := role.DeepCopy()
			original := role.DeepCopy()

			var podNames []string

			// Create list of pods names
			for _, deploy := range podRoleController.Spec.Deployments {

				listOptions := metav1.ListOptions{
					LabelSelector: podRoleController.Spec.LabelSelector + "=" + deploy,
					FieldSelector: "status.phase=Running",
				}

				pods, err := r.KubeClient.CoreV1().Pods(original.Namespace).List(ctx, listOptions)
				if err != nil {

					if errors.IsNotFound(err) {
						// Request object not found, could have been deleted after reconcile request.
						// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
						// Return and don't requeue
						r.Log.Info("No pods found for deployment " + req.Name + " in namespace " + req.Namespace + " was not found")
						return reconcile.Result{}, client.IgnoreNotFound(err)
					}
					return reconcile.Result{}, err
				}

				if len(pods.Items) != 0 {
					for _, pod := range pods.Items {
						podNames = append(podNames, pod.Name)
					}
				}

			}

			for _, rule := range original.Rules {
				for _, resource := range rule.Resources {
					_, resourceFound := utilities.Find(podRoleController.Spec.Resources, resource)

					if resourceFound {
						// Determine of the current resource Names match the current pod names
						if !reflect.DeepEqual(rule.ResourceNames, podNames) {
							rule.ResourceNames = podNames
						}

					} else {

						if len(rule.ResourceNames) != 0 {
							r.Log.Info("Clearing resource names...")
							rule.ResourceNames = make([]string, 0)
						}

					}
					// If there are no Pods found clear resource names
					if len(podNames) == 0 {
						if len(rule.ResourceNames) != 0 {
							r.Log.Info("Clearing resource names...")
							rule.ResourceNames = make([]string, 0)
						}
					}

				}
				rules = append(rules, rule)
			}

			copy.Rules = rules
			err = r.updateRole(ctx, copy, original, req.Name)
			if err != nil {
				return ctrl.Result{}, err
			}

			r.Log.Info("Reconciliation completed for podRoleController: " + req.Name + " in namespace " + req.Namespace)
		}

	}

	return ctrl.Result{}, nil
}

func (r *PodRoleControllerReconciler) updateRules(ctx context.Context, role *rbacv1.Role, deploy string, namespacedName types.NamespacedName) ([]rbacv1.PolicyRule, error) {

	rules := []rbacv1.PolicyRule{}

	listOptions := metav1.ListOptions{
		LabelSelector: podRoleController.Spec.LabelSelector + "=" + deploy,
		FieldSelector: "status.phase=Running",
	}

	pods, err := r.KubeClient.CoreV1().Pods(role.Namespace).List(ctx, listOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.Info("No pod founds for deployment " + deploy + " in namespace " + role.Namespace + " was not found")
			return nil, nil
		}
		return nil, err
	}

	for _, rule := range role.Rules {
		for _, resource := range rule.Resources {
			_, resourceFound := utilities.Find(podRoleController.Spec.Resources, resource)

			if resourceFound {

				for _, pod := range pods.Items {
					_, podFound := utilities.Find(rule.ResourceNames, pod.Name)

					if !podFound {
						if pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp == nil {
							r.Log.Info("Adding resource name " + pod.Name)
							rule.ResourceNames = append(rule.ResourceNames, pod.Name)
						}

					} else {

						// If pod found and it was recently deleted, lets remove from resource name list
						if pod.DeletionTimestamp != nil {
							r.Log.Info("Clearing resource name " + pod.Name)
							rule.ResourceNames = utilities.Remove(rule.ResourceNames, pod.Name)
						}
					}
				}
			}
		}
		rules = append(rules, rule)

	}

	return rules, nil
}

func (r *PodRoleControllerReconciler) updateRole(ctx context.Context, copy, original *rbacv1.Role, refName string) error {

	// namespacedName := types.NamespacedName{Name: podRoleController.Name, Namespace: podRoleController.Namespace}

	if !reflect.DeepEqual(original.Rules, copy.Rules) {

		err := r.Client.Patch(ctx, copy, client.StrategicMergeFrom(original))
		if err != nil {
			return fmt.Errorf("could not write to ROLE: %+v", err)
		}
		r.Log.Info("Patching Role completed for: " + copy.Name)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodRoleControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PodRoleController{}).
		Watches(&source.Kind{Type: &rbacv1.Role{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &appsv1.ReplicaSet{}}, &handler.EnqueueRequestForObject{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
