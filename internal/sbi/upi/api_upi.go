package upi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	smf_context "github.com/free5gc/smf/internal/context"
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

	ctx, cancel := context.WithCancel(context.Background())
	// set only if not set before
	if smf_context.SMF_Self().PFCPCancelFunc == nil {
		smf_context.SMF_Self().PFCPCancelFunc = cancel
	}
	for _, upf := range upi.UPFs {
		// only register new ones - same logic as in init.go L271
		if upf.UPF.UPFStatus == smf_context.NotAssociated {
			go association.ToBeAssociatedWithUPF(ctx, upf.UPF)
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
