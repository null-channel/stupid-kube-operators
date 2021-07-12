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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type GamePhase string

const (
	GamePhasePending  = GamePhase("Pending")
	GamePhaseCreating = GamePhase("ClusterCreating")
	GamePhaseActive   = GamePhase("OperatorInstalling")
	GamePhaseFinished = GamePhase("Provisioned")
)

// GameSpec defines the desired state of Game
type GameSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Game. Edit game_types.go to remove/update
	Solution NamespacedName `json:"solution,omitempty"`
}

// GameStatus defines the observed state of Game
type GameStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase           string `json:"phase,omitempty"`
	Current         string `json:"current,omitempty"`
	NumberOfGuesses int    `json:"numberOfGuesses,omitempty"`
	Status          string `json:"status,omitempty"`
}

func (c *GameStatus) SetTypedPhase(p GamePhase) {
	c.Phase = string(p)
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Game is the Schema for the games API
type Game struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameSpec   `json:"spec,omitempty"`
	Status GameStatus `json:"status,omitempty"`
}

type NamespacedName struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

func (n NamespacedName) ToObjectKey() client.ObjectKey {
	return client.ObjectKey{
		Namespace: n.Namespace,
		Name:      n.Name,
	}
}

//+kubebuilder:object:root=true

// GameList contains a list of Game
type GameList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Game `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Game{}, &GameList{})
}
