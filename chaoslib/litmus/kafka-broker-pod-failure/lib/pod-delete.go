package lib

import (
	"strconv"
	"strings"
	"time"

	clients "github.com/litmuschaos/litmus-go/pkg/clients"
	"github.com/litmuschaos/litmus-go/pkg/events"
	experimentTypes "github.com/litmuschaos/litmus-go/pkg/kafka/types"
	"github.com/litmuschaos/litmus-go/pkg/log"
	"github.com/litmuschaos/litmus-go/pkg/probe"
	"github.com/litmuschaos/litmus-go/pkg/status"
	"github.com/litmuschaos/litmus-go/pkg/types"
	"github.com/litmuschaos/litmus-go/pkg/utils/annotation"
	"github.com/litmuschaos/litmus-go/pkg/utils/common"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//PreparePodDelete contains the prepration steps before chaos injection
func PreparePodDelete(experimentsDetails *experimentTypes.ExperimentDetails, clients clients.ClientSets, resultDetails *types.ResultDetails, eventsDetails *types.EventDetails, chaosDetails *types.ChaosDetails) error {

	//Waiting for the ramp time before chaos injection
	if experimentsDetails.ChaoslibDetail.RampTime != 0 {
		log.Infof("[Ramp]: Waiting for the %vs ramp time before injecting chaos", experimentsDetails.ChaoslibDetail.RampTime)
		common.WaitForDuration(experimentsDetails.ChaoslibDetail.RampTime)
	}

	switch strings.ToLower(experimentsDetails.ChaoslibDetail.Sequence) {
	case "serial":
		if err := injectChaosInSerialMode(experimentsDetails, clients, chaosDetails, eventsDetails, resultDetails); err != nil {
			return err
		}
	case "parallel":
		if err := injectChaosInParallelMode(experimentsDetails, clients, chaosDetails, eventsDetails, resultDetails); err != nil {
			return err
		}
	default:
		return errors.Errorf("%v sequence is not supported", experimentsDetails.ChaoslibDetail.Sequence)
	}

	//Waiting for the ramp time after chaos injection
	if experimentsDetails.ChaoslibDetail.RampTime != 0 {
		log.Infof("[Ramp]: Waiting for the %vs ramp time after injecting chaos", experimentsDetails.ChaoslibDetail.RampTime)
		common.WaitForDuration(experimentsDetails.ChaoslibDetail.RampTime)
	}
	return nil
}

// injectChaosInSerialMode delete the kafka broker pods in serial mode(one by one)
func injectChaosInSerialMode(experimentsDetails *experimentTypes.ExperimentDetails, clients clients.ClientSets, chaosDetails *types.ChaosDetails, eventsDetails *types.EventDetails, resultDetails *types.ResultDetails) error {

	// run the probes during chaos
	if len(resultDetails.ProbeDetails) != 0 {
		if err := probe.RunProbes(chaosDetails, clients, resultDetails, "DuringChaos", eventsDetails); err != nil {
			return err
		}
	}

	GracePeriod := int64(0)
	//ChaosStartTimeStamp contains the start timestamp, when the chaos injection begin
	ChaosStartTimeStamp := time.Now()
	duration := int(time.Since(ChaosStartTimeStamp).Seconds())

	for duration < experimentsDetails.ChaoslibDetail.ChaosDuration {
		// Get the target pod details for the chaos execution
		// if the target pod is not defined it will derive the random target pod list using pod affected percentage
		if experimentsDetails.KafkaBroker == "" && chaosDetails.AppDetail.Label == "" {
			return errors.Errorf("please provide one of the appLabel or KAFKA_BROKER")
		}
		targetPodList, err := common.GetPodList(experimentsDetails.KafkaBroker, experimentsDetails.ChaoslibDetail.PodsAffectedPerc, clients, chaosDetails)
		if err != nil {
			return err
		}

		for _, pod := range targetPodList.Items {
			parentName, err := annotation.GetParentName(clients, pod, chaosDetails)
			if err != nil {
				return err
			}
			common.SetParentName(parentName, chaosDetails)
		}

		for _, target := range chaosDetails.ParentsResources {
			common.SetTargets(target, "targeted", chaosDetails.AppDetail.Kind, chaosDetails)
		}

		if experimentsDetails.ChaoslibDetail.EngineName != "" {
			msg := "Injecting " + experimentsDetails.ExperimentName + " chaos on application pod"
			types.SetEngineEventAttributes(eventsDetails, types.ChaosInject, msg, "Normal", chaosDetails)
			events.GenerateEvents(eventsDetails, clients, chaosDetails, "ChaosEngine")
		}

		//Deleting the application pod
		for _, pod := range targetPodList.Items {

			log.InfoWithValues("[Info]: Killing the following pods", logrus.Fields{
				"PodName": pod.Name})

			if experimentsDetails.ChaoslibDetail.Force {
				err = clients.KubeClient.CoreV1().Pods(experimentsDetails.ChaoslibDetail.AppNS).Delete(pod.Name, &v1.DeleteOptions{GracePeriodSeconds: &GracePeriod})
			} else {
				err = clients.KubeClient.CoreV1().Pods(experimentsDetails.ChaoslibDetail.AppNS).Delete(pod.Name, &v1.DeleteOptions{})
			}
			if err != nil {
				return err
			}

			switch chaosDetails.Randomness {
			case true:
				if err := common.RandomInterval(experimentsDetails.ChaoslibDetail.ChaosInterval); err != nil {
					return err
				}
			default:
				//Waiting for the chaos interval after chaos injection
				if experimentsDetails.ChaoslibDetail.ChaosInterval != "" {
					log.Infof("[Wait]: Wait for the chaos interval %vs", experimentsDetails.ChaoslibDetail.ChaosInterval)
					waitTime, _ := strconv.Atoi(experimentsDetails.ChaoslibDetail.ChaosInterval)
					common.WaitForDuration(waitTime)
				}
			}

			//Verify the status of pod after the chaos injection
			log.Info("[Status]: Verification for the recreation of application pod")
			if err = status.CheckApplicationStatus(experimentsDetails.ChaoslibDetail.AppNS, experimentsDetails.ChaoslibDetail.AppLabel, experimentsDetails.ChaoslibDetail.Timeout, experimentsDetails.ChaoslibDetail.Delay, clients); err != nil {
				return err
			}
		}
		duration = int(time.Since(ChaosStartTimeStamp).Seconds())
	}
	log.Infof("[Completion]: %v chaos is done", experimentsDetails.ExperimentName)

	return nil
}

// injectChaosInParallelMode delete the kafka broker pods in parallel mode (all at once)
func injectChaosInParallelMode(experimentsDetails *experimentTypes.ExperimentDetails, clients clients.ClientSets, chaosDetails *types.ChaosDetails, eventsDetails *types.EventDetails, resultDetails *types.ResultDetails) error {

	// run the probes during chaos
	if len(resultDetails.ProbeDetails) != 0 {
		if err := probe.RunProbes(chaosDetails, clients, resultDetails, "DuringChaos", eventsDetails); err != nil {
			return err
		}
	}

	GracePeriod := int64(0)
	//ChaosStartTimeStamp contains the start timestamp, when the chaos injection begin
	ChaosStartTimeStamp := time.Now()
	duration := int(time.Since(ChaosStartTimeStamp).Seconds())

	for duration < experimentsDetails.ChaoslibDetail.ChaosDuration {
		// Get the target pod details for the chaos execution
		// if the target pod is not defined it will derive the random target pod list using pod affected percentage
		if experimentsDetails.KafkaBroker == "" && chaosDetails.AppDetail.Label == "" {
			return errors.Errorf("please provide one of the appLabel or KAFKA_BROKER")
		}
		targetPodList, err := common.GetPodList(experimentsDetails.KafkaBroker, experimentsDetails.ChaoslibDetail.PodsAffectedPerc, clients, chaosDetails)
		if err != nil {
			return err
		}

		for _, pod := range targetPodList.Items {
			parentName, err := annotation.GetParentName(clients, pod, chaosDetails)
			if err != nil {
				return err
			}
			common.SetParentName(parentName, chaosDetails)
		}

		for _, target := range chaosDetails.ParentsResources {
			common.SetTargets(target, "targeted", chaosDetails.AppDetail.Kind, chaosDetails)
		}

		if experimentsDetails.ChaoslibDetail.EngineName != "" {
			msg := "Injecting " + experimentsDetails.ExperimentName + " chaos on application pod"
			types.SetEngineEventAttributes(eventsDetails, types.ChaosInject, msg, "Normal", chaosDetails)
			events.GenerateEvents(eventsDetails, clients, chaosDetails, "ChaosEngine")
		}

		//Deleting the application pod
		for _, pod := range targetPodList.Items {

			log.InfoWithValues("[Info]: Killing the following pods", logrus.Fields{
				"PodName": pod.Name})

			if experimentsDetails.ChaoslibDetail.Force {
				err = clients.KubeClient.CoreV1().Pods(experimentsDetails.ChaoslibDetail.AppNS).Delete(pod.Name, &v1.DeleteOptions{GracePeriodSeconds: &GracePeriod})
			} else {
				err = clients.KubeClient.CoreV1().Pods(experimentsDetails.ChaoslibDetail.AppNS).Delete(pod.Name, &v1.DeleteOptions{})
			}
			if err != nil {
				return err
			}
		}

		switch chaosDetails.Randomness {
		case true:
			if err := common.RandomInterval(experimentsDetails.ChaoslibDetail.ChaosInterval); err != nil {
				return err
			}
		default:
			//Waiting for the chaos interval after chaos injection
			if experimentsDetails.ChaoslibDetail.ChaosInterval != "" {
				log.Infof("[Wait]: Wait for the chaos interval %vs", experimentsDetails.ChaoslibDetail.ChaosInterval)
				waitTime, _ := strconv.Atoi(experimentsDetails.ChaoslibDetail.ChaosInterval)
				common.WaitForDuration(waitTime)
			}
		}

		//Verify the status of pod after the chaos injection
		log.Info("[Status]: Verification for the recreation of application pod")
		if err = status.CheckApplicationStatus(experimentsDetails.ChaoslibDetail.AppNS, experimentsDetails.ChaoslibDetail.AppLabel, experimentsDetails.ChaoslibDetail.Timeout, experimentsDetails.ChaoslibDetail.Delay, clients); err != nil {
			return err
		}

		duration = int(time.Since(ChaosStartTimeStamp).Seconds())
	}

	log.Infof("[Completion]: %v chaos is done", experimentsDetails.ExperimentName)

	return nil
}
