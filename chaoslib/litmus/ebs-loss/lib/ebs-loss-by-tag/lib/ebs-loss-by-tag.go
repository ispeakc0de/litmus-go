package lib

import (
	"strings"

	ebsloss "github.com/litmuschaos/litmus-go/chaoslib/litmus/ebs-loss/lib"

	clients "github.com/litmuschaos/litmus-go/pkg/clients"
	experimentTypes "github.com/litmuschaos/litmus-go/pkg/kube-aws/ebs-loss/types"
	"github.com/litmuschaos/litmus-go/pkg/log"
	"github.com/litmuschaos/litmus-go/pkg/types"
	"github.com/litmuschaos/litmus-go/pkg/utils/common"
	"github.com/pkg/errors"
)

//PrepareEBSLossByTag contains the prepration and injection steps for the experiment
func PrepareEBSLossByTag(experimentsDetails *experimentTypes.ExperimentDetails, clients clients.ClientSets, resultDetails *types.ResultDetails, eventsDetails *types.EventDetails, chaosDetails *types.ChaosDetails) error {

	var err error
	//Waiting for the ramp time before chaos injection
	if experimentsDetails.RampTime != 0 {
		log.Infof("[Ramp]: Waiting for the %vs ramp time before injecting chaos", experimentsDetails.RampTime)
		common.WaitForDuration(experimentsDetails.RampTime)
	}

	targetEBSVolumeIDList := common.CalculateVolumeAffPerc(experimentsDetails.VolumeAffectedPerc, experimentsDetails.TargetVolumeIDList)
	log.Infof("[Chaos]:Number of volumes targeted: %v", len(targetEBSVolumeIDList))

	switch strings.ToLower(experimentsDetails.Sequence) {
	case "serial":
		if err = ebsloss.InjectChaosInSerialMode(experimentsDetails, targetEBSVolumeIDList, clients, resultDetails, eventsDetails, chaosDetails); err != nil {
			return err
		}
	case "parallel":
		if err = ebsloss.InjectChaosInParallelMode(experimentsDetails, targetEBSVolumeIDList, clients, resultDetails, eventsDetails, chaosDetails); err != nil {
			return err
		}
	default:
		return errors.Errorf("%v sequence is not supported", experimentsDetails.Sequence)
	}
	//Waiting for the ramp time after chaos injection
	if experimentsDetails.RampTime != 0 {
		log.Infof("[Ramp]: Waiting for the %vs ramp time after injecting chaos", experimentsDetails.RampTime)
		common.WaitForDuration(experimentsDetails.RampTime)
	}
	return nil
}
