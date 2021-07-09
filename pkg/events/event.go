package events

import (
	"time"

	clients "github.com/litmuschaos/litmus-go/pkg/clients"
	apiv1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientTypes "k8s.io/apimachinery/pkg/types"
)

// EventDetails is for collecting all the events-related details
type EventDetails struct {
	Name           string
	Namespace      string
	Kind           string
	Message        string
	Reason         string
	ResourceName   string
	ResourceUID    clientTypes.UID
	Type           string
	Source         string
	ExperimentName string
}

//CreateEvents create the events in the desired resource
func (e *EventDetails) CreateEvents(clients clients.ClientSets) error {
	events := &apiv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.Name,
			Namespace: e.Namespace,
		},
		Source: apiv1.EventSource{
			Component: e.Source,
		},
		Message:        e.Message,
		Reason:         e.Reason,
		Type:           e.Type,
		Count:          1,
		FirstTimestamp: metav1.Time{Time: time.Now()},
		LastTimestamp:  metav1.Time{Time: time.Now()},
		InvolvedObject: apiv1.ObjectReference{
			APIVersion: "litmuschaos.io/v1alpha1",
			Kind:       e.Kind,
			Name:       e.ResourceName,
			Namespace:  e.Namespace,
			UID:        e.ResourceUID,
		},
	}

	_, err := clients.KubeClient.CoreV1().Events(e.Namespace).Create(events)
	return err
}

//GenerateEvents update the events and increase the count by 1, if already present
// else it will create a new event
func (e *EventDetails) GenerateEvents(clients clients.ClientSets) error {

	switch e.Kind {
	case "ChaosResult":
		e.Name = e.Reason + e.Source
		if err := e.CreateEvents(clients); err != nil {
			return err
		}
	case "ChaosEngine":
		e.Name = e.Reason + e.ExperimentName + string(e.ResourceUID)
		event, err := clients.KubeClient.CoreV1().Events(e.Namespace).Get(e.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				if err := e.CreateEvents(clients); err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			event.LastTimestamp = metav1.Time{Time: time.Now()}
			event.Count = event.Count + 1
			event.Source.Component = e.Source
			event.Message = e.Message
			_, err = clients.KubeClient.CoreV1().Events(e.Namespace).Update(event)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
