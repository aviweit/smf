package upi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	smf_context "github.com/free5gc/smf/internal/context"
	"github.com/free5gc/smf/internal/logger"
	"github.com/free5gc/smf/pkg/association"
	"github.com/free5gc/smf/pkg/factory"
	"github.com/free5gc/util/httpwrapper"
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

	for _, upf := range upi.UPFs {
		// only register new ones
		if upf.UPF.UPFStatus == smf_context.NotAssociated {
			go association.ToBeAssociatedWithUPF(smf_context.SMF_Self().Ctx, upf.UPF)
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

func DeleteUpNode(c *gin.Context) {
	req := httpwrapper.NewRequest(c.Request, nil)
	req.Params["upfRef"] = c.Params.ByName("upfRef")

	upfRef := req.Params["upfRef"]
	upi := smf_context.SMF_Self().UserPlaneInformation
	found := false

	upNode, ok := upi.UPNodes[upfRef]
	if ok {
		found = true
		if upNode.Type == "UPF" {
			logger.InitLog.Infof("UPF [%s] FOUND. Release its sessions.\n", upfRef)
			association.ReleaseAllResourcesOfUPF(smf_context.SMF_Self().Ctx, upNode.UPF)
			logger.InitLog.Infof("UPF [%s] FOUND. Remove its NodeID.\n", upfRef)
			smf_context.RemoveUPFNodeByNodeID(upNode.UPF.NodeID)
		}
		logger.InitLog.Infof("UPF [%s] FOUND. Remove it from upi.UPNodes.\n", upfRef)
		delete(upi.UPNodes, upfRef)
	}
	_, ok2 := upi.UPFs[upfRef]
	if ok2 {
		logger.InitLog.Infof("UPF [%s] FOUND. Remove it from upi.UPFs.\n", upfRef)
		delete(upi.UPFs, upfRef)
	}

	if found {
		c.JSON(http.StatusNoContent, gin.H{})
	} else {
		c.JSON(http.StatusNotFound, gin.H{})
	}
}

func DeleteLink(c *gin.Context) {
	req := httpwrapper.NewRequest(c.Request, nil)
	req.Params["upfRef"] = c.Params.ByName("upfRef")

	upfRef := req.Params["upfRef"]
	upi := smf_context.SMF_Self().UserPlaneInformation
	upi.LinksDeleteFromUpfName(upfRef)

	// TODO: support 404
	c.JSON(http.StatusNoContent, gin.H{})
}
