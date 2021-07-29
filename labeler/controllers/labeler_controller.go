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

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	nulllabelerv1 "github.com/null-channel/stupid-kube-operators/labeler/api/v1"
)

// LabelerReconciler reconciles a Labeler object
type LabelerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=nulllabeler.thenullchannel.dev,resources=labelers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nulllabeler.thenullchannel.dev,resources=labelers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nulllabeler.thenullchannel.dev,resources=labelers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Labeler object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *LabelerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	labeler := &nulllabelerv1.Labeler{}

	if err := r.Client.Get(ctx, req.NamespacedName, labeler); err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	podList := &core.PodList{}

	r.Client.List(context.Background(), podList)

	for _, pod := range podList.Items {
		if pod.Labels == nil {
			pod.Labels = map[string]string{"null-labeler": labeler.Spec.Label}
		} else {
			pod.Labels["null-labeler"] = labeler.Spec.Label
		}
		r.Update(ctx, &pod) //TODO: Care about the error returned here
	}

	// your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LabelerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nulllabelerv1.Labeler{}).
		Watches(
			&source.Kind{Type: &core.Pod{}},
			handler.EnqueueRequestsFromMapFunc(r.GetAll),
		).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		// For().
		Complete(r)
}

func (r *LabelerReconciler) GetAll(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}

	labelerList := nulllabelerv1.LabelerList{}
	r.Client.List(context.Background(), &labelerList)

	for _, labeler := range labelerList.Items {
		result = append(result, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: labeler.Namespace, Name: labeler.Name}})
	}

	return result
}
