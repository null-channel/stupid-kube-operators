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

	"github.com/tjarratt/babble"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kerrors "k8s.io/apimachinery/pkg/util/errors"

	nullgamev1 "github.com/null-channel/stupid-kube-operators/game/api/v1"
)

var (
	MaxGuesses = 5
)

// GameReconciler reconciles a Game object
type GameReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=nullgame.thenullchannel.dev,resources=games,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nullgame.thenullchannel.dev,resources=games/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nullgame.thenullchannel.dev,resources=games/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Game object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *GameReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ret ctrl.Result, reterr error) {
	_ = log.FromContext(ctx)

	game := &nullgamev1.Game{}

	if err := r.Client.Get(ctx, req.NamespacedName, game); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Get phrase
	phrase := corev1.Secret{}
	if err := r.Client.Get(ctx, game.Spec.Solution.ToObjectKey(), game); err != nil {
		if apierrors.IsNotFound(err) {
			// If the Secret is not created yet, it means its a new game, Lets create a new one!

			// Create new phrase
			// TODO: update to use real phrases and not random words
			babbler := babble.NewBabbler()
			babbler.Separator = " "
			phrase.Data = map[string][]byte{"result": []byte(babbler.Babble())}
			phrase.Name = game.Name + "Secret"
			game.Spec.Solution = nullgamev1.NamespacedName(client.ObjectKeyFromObject(&phrase))

			return ctrl.Result{}, nil
		}
	}

	guesses := &[]nullgamev1.Guess{}

	// ensure phase is always patched
	// We want to make sure no matter where we fail out, we update the status with the latest.

	patchHelper, err := patch.NewHelper(game, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		// Always reconcile the Status.Phase field.
		r.reconcilePhase(game, guesses)

		// Always attempt to Patch the Cluster object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{}
		if reterr == nil {
			patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
		}
		if err := patchHelper.Patch(ctx, game, patchOpts...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return ctrl.Result{}, reterr
}

func (r *GameReconciler) reconcilePhase(game *nullgamev1.Game, guesses *[]nullgamev1.Guess) {

	game.Status.NumberOfGuesses = len(*guesses)

	if game.Status.Phase == "" {
		game.Status.SetTypedPhase(nullgamev1.GamePhasePending)
	}

	if game.Status.NumberOfGuesses > 0 {
		game.Status.SetTypedPhase(nullgamev1.GamePhaseActive)
	}

	if game.Status.NumberOfGuesses > MaxGuesses {
		game.Status.SetTypedPhase(nullgamev1.GamePhaseFinished)
	}

}

// SetupWithManager sets up the controller with the Manager.
func (r *GameReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nullgamev1.Game{}).
		Complete(r)
}
