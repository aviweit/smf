package upi

import (
	"fmt"
	"github.com/free5gc/pfcp"
	"github.com/free5gc/pfcp/pfcpType"
	smf_context "github.com/free5gc/smf/internal/context"
	"github.com/free5gc/smf/internal/logger"
	"github.com/free5gc/smf/internal/pfcp/message"
	"github.com/free5gc/smf/pkg/factory"
	"github.com/free5gc/util/httpwrapper"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetUpNodes(c *gin.Context) {
	upi := smf_context.SMF_Self().UserPlaneInformation
	json := upi.UpNodesToConfiguration()

	httpResponse := &httpwrapper.Response{
		Header: nil,
		Status: http.StatusOK,
		Body:   json,
	}
	c.JSON(httpResponse.Status, httpResponse.Body)
}

func GetLinks(c *gin.Context) {
	upi := smf_context.SMF_Self().UserPlaneInformation
	json := upi.LinksToConfiguration()

	httpResponse := &httpwrapper.Response{
		Header: nil,
		Status: http.StatusOK,
		Body:   json,
	}
	c.JSON(httpResponse.Status, httpResponse.Body)
}

func PostUpNodes(c *gin.Context) {
	upi := smf_context.SMF_Self().UserPlaneInformation
	var json factory.UserPlaneInformation
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	upi.UpNodesFromConfiguration(&json)

	ctx, cancel := context.WithCancel(context.Background())
	// set only if not set before
	if smf_context.SMF_Self().PFCPCancelFunc == nil {
		smf_context.SMF_Self().PFCPCancelFunc = cancel
	}
	for _, upf := range upi.UPFs {
		// only register new ones - same logic as in init.go L271
		if upf.UPF.UPFStatus == smf_context.NotAssociated {
			go toBeAssociatedWithUPF(ctx, upNode.UPF)
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

func PostLinks(c *gin.Context) {
	upi := smf_context.SMF_Self().UserPlaneInformation
	var json factory.UserPlaneInformation
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	upi.LinksFromConfiguration(&json)
	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

// TODO: where should it belong ?
func setupPfcpAssociation(upf *smf_context.UPF, upfStr string) error {
	logger.AppLog.Infof("Sending PFCP Association Request to UPF%s", upfStr)

	resMsg, err := message.SendPfcpAssociationSetupRequest(upf.NodeID)
	if err != nil {
		return err
	}

	rsp := resMsg.PfcpMessage.Body.(pfcp.PFCPAssociationSetupResponse)

	if rsp.Cause == nil || rsp.Cause.CauseValue != pfcpType.CauseRequestAccepted {
		return fmt.Errorf("received PFCP Association Setup Not Accepted Response from UPF%s", upfStr)
	}

	nodeID := rsp.NodeID
	if nodeID == nil {
		return fmt.Errorf("pfcp association needs NodeID")
	}

	logger.AppLog.Infof("Received PFCP Association Setup Accepted Response from UPF%s", upfStr)

	upf.UPFStatus = smf_context.AssociatedSetUpSuccess

	if rsp.UserPlaneIPResourceInformation != nil {
		upf.UPIPInfo = *rsp.UserPlaneIPResourceInformation

		logger.AppLog.Infof("UPF(%s)[%s] setup association",
			upf.NodeID.ResolveNodeIdToIp().String(), upf.UPIPInfo.NetworkInstance)
	}

	return nil
}
