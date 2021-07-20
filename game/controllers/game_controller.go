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

	"github.com/pkg/errors"
	"github.com/tjarratt/babble"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

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

//+kubebuilder:rbac:groups=nullgame.thenullchannel.dev,resources=guesses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nullgame.thenullchannel.dev,resources=guesses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nullgame.thenullchannel.dev,resources=games,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nullgame.thenullchannel.dev,resources=games/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nullgame.thenullchannel.dev,resources=games/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch
//+kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch

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

	original := game.DeepCopy()

	gameFinalizerName := "nullgame.nullchannel.io/finalizer"

	if game.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(game.GetFinalizers(), gameFinalizerName) {
			controllerutil.AddFinalizer(game, gameFinalizerName)
			if err := r.Update(ctx, game); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if containsString(game.GetFinalizers(), gameFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteExternalResources(game); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(game, gameFinalizerName)
			if err := r.Update(ctx, game); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	guessList := &nullgamev1.GuessList{}
	phrase := &corev1.Secret{}

	patchHelper, err := patch.NewHelper(original, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// ensure phase is always patched
	// We want to make sure no matter where we fail out, we update the status with the latest.
	defer func() {
		// Always reconcile the Status.Phase field.
		r.reconcilePhase(game, &guessList.Items, string(phrase.Data["phrase"]))

		patchOpts := []patch.Option{}
		if reterr == nil {
			patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
		}
		if err := patchHelper.Patch(ctx, game, patchOpts...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}

		// or you could do this and fail at it. All of these methods faild for one reason or another. ether they would not patch status or would infinate loop me.
		/*
			//err := r.Patch(context.Background(), game, client.MergeFrom(original))
			err := r.Update(context.Background(), game)
			if err != nil {
				reterr = errors.Wrap(err, "Failed to update status")
			}
			err = r.Status().Patch(context.Background(), game, client.Merge) //.Update(context.Background(), game)
			if err != nil {
				reterr = errors.Wrap(err, "Failed to update status")
			}
		*/
	}()

	// Get phrase
	if err := r.Client.Get(ctx, game.Spec.Solution.ToObjectKey(), phrase); err != nil {
		if apierrors.IsNotFound(err) {
			// If the Secret is not created yet, it means its a new game, Lets create a new one!
			fmt.Println("Game has no phrase! creating a new phrase!")
			// Create new phrase
			// TODO: update to use real phrases and not random words
			babbler := babble.NewBabbler()
			babbler.Separator = " "
			phrase.Data = map[string][]byte{"phrase": []byte(babbler.Babble())}
			phrase.Name = game.Name + "-secret"
			phrase.Namespace = metav1.NamespaceDefault
			game.Spec.Solution = nullgamev1.NamespacedName(client.ObjectKeyFromObject(phrase))
			err = r.Client.Create(ctx, phrase)
			if err != nil {
				reterr = errors.Wrapf(reterr, "failed to create secret: %s", err)
			}
		}
	}

	r.Client.List(ctx, guessList, &client.MatchingLabels{"null-game": game.Name})

	return ctrl.Result{}, reterr
}

func (r *GameReconciler) reconcilePhase(game *nullgamev1.Game, guesses *[]nullgamev1.Guess, phrase string) {

	game.Status.NumberOfGuesses = len(*guesses)

	game.Status.SetCurrent(guesses, phrase)

	if game.Status.Current != phrase && len(*guesses) > MaxGuesses {
		game.Status.Current = "FAILED"
	}

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
		Watches(
			&source.Kind{Type: &nullgamev1.Guess{}},
			handler.EnqueueRequestsFromMapFunc(r.GuessToGame),
		).
		Complete(r)
}

func (r *GameReconciler) GuessToGame(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}

	guess, ok := o.(*nullgamev1.Guess)
	if !ok {
		fmt.Println("Failed to Map Guess to Game")
		return result
	}
	gameKey := client.ObjectKey{Namespace: guess.Namespace, Name: guess.Spec.Game}

	result = append(result, ctrl.Request{NamespacedName: gameKey})

	return result
}

func (r *GameReconciler) deleteExternalResources(game *nullgamev1.Game) error {
	//
	// delete any external resources associated with the game
	// in our case, we want to delete all the guesses made for this game.
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.

	fmt.Println("Game deleted, cleaning up the game")

	guessList := &nullgamev1.GuessList{}
	r.Client.List(context.Background(), guessList, &client.MatchingLabels{"null-game": game.Name})

	for _, guess := range guessList.Items {
		r.Client.Delete(context.Background(), &guess)
	}

	return nil
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
