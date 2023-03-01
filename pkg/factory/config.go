/*
 * SMF Configuration Factory
 */

package factory

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"

	"github.com/free5gc/openapi/models"
	logger_util "github.com/free5gc/util/logger"
)

const (
	SMF_EXPECTED_CONFIG_VERSION        = "1.0.2"
	UE_ROUTING_EXPECTED_CONFIG_VERSION = "1.0.1"
	SMF_DEFAULT_IPV4                   = "127.0.0.2"
	SMF_DEFAULT_PORT                   = 8000
)

type Config struct {
	Info          *Info               `yaml:"info" valid:"required"`
	Configuration *Configuration      `yaml:"configuration" valid:"required"`
	Logger        *logger_util.Logger `yaml:"logger" valid:"optional"`
}

func (c *Config) Validate() (bool, error) {
	if info := c.Info; info != nil {
		if result, err := info.validate(); err != nil {
			return result, err
		}
	}

	if configuration := c.Configuration; configuration != nil {
		if result, err := configuration.validate(); err != nil {
			return result, err
		}
	}

	if logger := c.Logger; logger != nil {
		if result, err := logger.Validate(); err != nil {
			return result, err
		}
	}

	result, err := govalidator.ValidateStruct(c)
	return result, appendInvalid(err)
}

type Info struct {
	Version     string `yaml:"version,omitempty" valid:"type(string)"`
	Description string `yaml:"description,omitempty" valid:"type(string)"`
}

func (i *Info) validate() (bool, error) {
	result, err := govalidator.ValidateStruct(i)
	return result, appendInvalid(err)
}

type Configuration struct {
	SmfName              string               `yaml:"smfName,omitempty" valid:"type(string),required"`
	Sbi                  *Sbi                 `yaml:"sbi,omitempty" valid:"required"`
	PFCP                 *PFCP                `yaml:"pfcp,omitempty" valid:"required"`
	NrfUri               string               `yaml:"nrfUri,omitempty" valid:"url,required"`
	UserPlaneInformation UserPlaneInformation `yaml:"userplaneInformation" valid:"required"`
	ServiceNameList      []string             `yaml:"serviceNameList,omitempty" valid:"required"`
	SNssaiInfo           []SnssaiInfoItem     `yaml:"snssaiInfos,omitempty" valid:"required"`
	ULCL                 bool                 `yaml:"ulcl,omitempty" valid:"type(bool),optional"`
	PLMNList             []PlmnID             `yaml:"plmnList,omitempty"  valid:"optional"`
	Locality             string               `yaml:"locality,omitempty" valid:"type(string),optional"`
}

func (c *Configuration) validate() (bool, error) {
	if sbi := c.Sbi; sbi != nil {
		if result, err := sbi.validate(); err != nil {
			return result, err
		}
	}

	if pfcp := c.PFCP; pfcp != nil {
		if result, err := pfcp.validate(); err != nil {
			return result, err
		}
	}

	if userPlaneInformation := &c.UserPlaneInformation; userPlaneInformation != nil {
		if result, err := userPlaneInformation.validate(); err != nil {
			return result, err
		}
	}

	for index, serviceName := range c.ServiceNameList {
		switch {
		case serviceName == "nsmf-pdusession":
		case serviceName == "nsmf-event-exposure":
		case serviceName == "nsmf-oam":
		default:
			err := errors.New("Invalid serviceNameList[" + strconv.Itoa(index) + "]: " +
				serviceName + ", should be nsmf-pdusession, nsmf-event-exposure or nsmf-oam.")
			return false, err
		}
	}

	for _, snssaiInfo := range c.SNssaiInfo {
		if result, err := snssaiInfo.validate(); err != nil {
			return result, err
		}
	}

	if c.PLMNList != nil {
		for _, plmnId := range c.PLMNList {
			if result, err := plmnId.validate(); err != nil {
				return result, err
			}
		}
	}

	result, err := govalidator.ValidateStruct(c)
	return result, appendInvalid(err)
}

type SnssaiInfoItem struct {
	SNssai   *models.Snssai      `yaml:"sNssai" valid:"required"`
	DnnInfos []SnssaiDnnInfoItem `yaml:"dnnInfos" valid:"required"`
}

func (s *SnssaiInfoItem) validate() (bool, error) {
	if snssai := s.SNssai; snssai != nil {
		if result := (snssai.Sst >= 0 && snssai.Sst <= 255); !result {
			err := errors.New("Invalid sNssai.Sst: " + strconv.Itoa(int(snssai.Sst)) + ", should be in range 0~255.")
			return false, err
		}

		if result := govalidator.StringMatches(snssai.Sd, "^[0-9A-Fa-f]{6}$"); !result {
			err := errors.New("Invalid sNssai.Sd: " + snssai.Sd +
				", should be 3 bytes hex string and in range 000000~FFFFFF.")
			return false, err
		}
	}

	for _, dnnInfo := range s.DnnInfos {
		if result, err := dnnInfo.validate(); err != nil {
			return result, err
		}
	}
	result, err := govalidator.ValidateStruct(s)
	return result, appendInvalid(err)
}

type SnssaiDnnInfoItem struct {
	Dnn   string `yaml:"dnn" valid:"type(string),minstringlength(1),required"`
	DNS   *DNS   `yaml:"dns" valid:"required"`
	PCSCF *PCSCF `yaml:"pcscf,omitempty" valid:"optional"`
}

func (s *SnssaiDnnInfoItem) validate() (bool, error) {
	if dns := s.DNS; dns != nil {
		if result, err := dns.validate(); err != nil {
			return result, err
		}
	}

	if pcscf := s.PCSCF; pcscf != nil {
		if result, err := pcscf.validate(); err != nil {
			return result, err
		}
	}

	result, err := govalidator.ValidateStruct(s)
	return result, appendInvalid(err)
}

type Sbi struct {
	Scheme       string `yaml:"scheme" valid:"scheme,required"`
	Tls          *Tls   `yaml:"tls" valid:"optional"`
	RegisterIPv4 string `yaml:"registerIPv4,omitempty" valid:"host,optional"` // IP that is registered at NRF.
	// IPv6Addr string `yaml:"ipv6Addr,omitempty"`
	BindingIPv4 string `yaml:"bindingIPv4,omitempty" valid:"host,required"` // IP used to run the server in the node.
	Port        int    `yaml:"port,omitempty" valid:"port,optional"`
}

func (s *Sbi) validate() (bool, error) {
	govalidator.TagMap["scheme"] = govalidator.Validator(func(str string) bool {
		return str == "https" || str == "http"
	})

	if tls := s.Tls; tls != nil {
		if result, err := tls.validate(); err != nil {
			return result, err
		}
	}

	result, err := govalidator.ValidateStruct(s)
	return result, appendInvalid(err)
}

type Tls struct {
	Pem string `yaml:"pem,omitempty" valid:"type(string),minstringlength(1),required"`
	Key string `yaml:"key,omitempty" valid:"type(string),minstringlength(1),required"`
}

func (t *Tls) validate() (bool, error) {
	result, err := govalidator.ValidateStruct(t)
	return result, appendInvalid(err)
}

type PFCP struct {
	Addr string `yaml:"addr,omitempty" valid:"host,required"`
	Port uint16 `yaml:"port,omitempty" valid:"port,optional"`
	// interval at which PFCP Association Setup error messages are output.
	AlertInterval time.Duration `yaml:"associationSetupFailedAlertInterval,omitempty" valid:"type(time.Duration),optional"`
	RetryInterval time.Duration `yaml:"associationSetupFailedRetryInterval,omitempty" valid:"type(time.Duration),optional"`
	Heartbeat     PfcpHeartbeat `yaml:"heartbeat,omitempty" valid:"optional"`
}

type PfcpHeartbeat struct {
	Interval time.Duration `yaml:"interval,omitempty" valid:"type(time.Duration),optional"`
}

func (p *PFCP) validate() (bool, error) {
	result, err := govalidator.ValidateStruct(p)
	return result, appendInvalid(err)
}

type DNS struct {
	IPv4Addr string `yaml:"ipv4,omitempty" valid:"ipv4,required"`
	IPv6Addr string `yaml:"ipv6,omitempty" valid:"ipv6,optional"`
}

func (d *DNS) validate() (bool, error) {
	result, err := govalidator.ValidateStruct(d)
	return result, appendInvalid(err)
}

type PCSCF struct {
	IPv4Addr string `yaml:"ipv4,omitempty" valid:"ipv4,required"`
}

func (p *PCSCF) validate() (bool, error) {
	result, err := govalidator.ValidateStruct(p)
	return result, appendInvalid(err)
}

type UERoutingInfo struct {
	Members       []string       `yaml:"members" valid:"required"`
	AN            string         `yaml:"AN,omitempty" valid:"ipv4,optional"`
	Topology      []UPLink       `yaml:"topology" valid:"required"`
	SpecificPaths []SpecificPath `yaml:"specificPath,omitempty" valid:"optional"`
}

func (u *UERoutingInfo) validate() (bool, error) {
	for _, member := range u.Members {
		if result := govalidator.StringMatches(member, "imsi-[0-9]{5,15}$"); !result {
			err := errors.New("Invalid member (SUPI): " + member)
			return false, err
		}
	}

	for _, link := range u.Topology {
		if result, err := link.validate(); err != nil {
			return result, err
		}
	}

	for _, path := range u.SpecificPaths {
		if result, err := path.validate(); err != nil {
			return result, err
		}
	}

	result, err := govalidator.ValidateStruct(u)
	return result, appendInvalid(err)
}

// RouteProfID is string providing a Route Profile identifier.
type RouteProfID string

// RouteProfile maintains the mapping between RouteProfileID and ForwardingPolicyID of UPF
type RouteProfile struct {
	// Forwarding Policy ID of the route profile
	ForwardingPolicyID string `yaml:"forwardingPolicyID,omitempty" valid:"type(string),stringlength(1|255),required"`
}

func (r *RouteProfile) validate() (bool, error) {
	result, err := govalidator.ValidateStruct(r)
	return result, appendInvalid(err)
}

// PfdContent represents the flow of the application
type PfdContent struct {
	// Identifies a PFD of an application identifier.
	PfdID string `yaml:"pfdID,omitempty" valid:"type(string),minstringlength(1),required"`
	// Represents a 3-tuple with protocol, server ip and server port for
	// UL/DL application traffic.
	FlowDescriptions []string `yaml:"flowDescriptions,omitempty" valid:"optional"`
	// Indicates a URL or a regular expression which is used to match the
	// significant parts of the URL.
	Urls []string `yaml:"urls,omitempty" valid:"optional"`
	// Indicates an FQDN or a regular expression as a domain name matching
	// criteria.
	DomainNames []string `yaml:"domainNames,omitempty" valid:"optional"`
}

func (p *PfdContent) validate() (bool, error) {
	for _, flowDescription := range p.FlowDescriptions {
		if result := len(flowDescription) > 0; !result {
			err := errors.New("Invalid FlowDescription: " + flowDescription + ", should not be empty.")
			return false, err
		}
	}

	for _, url := range p.Urls {
		if result := govalidator.IsURL(url); !result {
			err := errors.New("Invalid Url: " + url + ", should be url.")
			return false, err
		}
	}

	for _, domainName := range p.DomainNames {
		if result := govalidator.IsDNSName(domainName); !result {
			err := errors.New("Invalid DomainName: " + domainName + ", should be domainName.")
			return false, err
		}
	}

	result, err := govalidator.ValidateStruct(p)
	return result, appendInvalid(err)
}

// PfdDataForApp represents the PFDs for an application identifier
type PfdDataForApp struct {
	// Identifier of an application.
	AppID string `yaml:"applicationId" valid:"type(string),minstringlength(1),required"`
	// PFDs for the application identifier.
	Pfds []PfdContent `yaml:"pfds" valid:"required"`
	// Caching time for an application identifier.
	CachingTime *time.Time `yaml:"cachingTime,omitempty" valid:"optional"`
}

func (p *PfdDataForApp) validate() (bool, error) {
	for _, pfd := range p.Pfds {
		if result, err := pfd.validate(); err != nil {
			return result, err
		}
	}

	result, err := govalidator.ValidateStruct(p)
	return result, appendInvalid(err)
}

type RoutingConfig struct {
	Info          *Info                        `yaml:"info" valid:"required"`
	UERoutingInfo map[string]UERoutingInfo     `yaml:"ueRoutingInfo" valid:"optional"`
	RouteProf     map[RouteProfID]RouteProfile `yaml:"routeProfile,omitempty" valid:"optional"`
	PfdDatas      []*PfdDataForApp             `yaml:"pfdDataForApp,omitempty" valid:"optional"`
}

func (r *RoutingConfig) Validate() (bool, error) {
	if info := r.Info; info != nil {
		if result, err := info.validate(); err != nil {
			return result, err
		}
	}

	for _, ueRoutingInfo := range r.UERoutingInfo {
		if result, err := ueRoutingInfo.validate(); err != nil {
			return result, err
		}
	}

	for _, routeProf := range r.RouteProf {
		if result, err := routeProf.validate(); err != nil {
			return result, err
		}
	}

	for _, pfdData := range r.PfdDatas {
		if result, err := pfdData.validate(); err != nil {
			return result, err
		}
	}

	result, err := govalidator.ValidateStruct(r)
	return result, appendInvalid(err)
}

// UserPlaneInformation describe core network userplane information
type UserPlaneInformation struct {
	UPNodes map[string]UPNode `json:"upNodes" yaml:"upNodes" valid:"required"`
	Links   []UPLink          `json:"links" yaml:"links" valid:"optional"`
}

func (u *UserPlaneInformation) validate() (bool, error) {
	for _, upNode := range u.UPNodes {
		if result, err := upNode.validate(); err != nil {
			return result, err
		}
	}

	for _, link := range u.Links {
		if result, err := link.validate(); err != nil {
			return result, err
		}
	}

	result, err := govalidator.ValidateStruct(u)
	return result, appendInvalid(err)
}

// UPNode represent the user plane node
type UPNode struct {
	Type                 string                 `json:"type" yaml:"type" valid:"upNodeType,required"`
	NodeID               string                 `json:"nodeID" yaml:"nodeID" valid:"host,optional"`
	Addr                 string                 `json:"addr" yaml:"addr" valid:"host,optional"`
	ANIP                 string                 `json:"anIP" yaml:"anIP" valid:"host,optional"`
	Dnn                  string                 `json:"dnn" yaml:"dnn" valid:"type(string),minstringlength(1),optional"`
	SNssaiInfos          []SnssaiUpfInfoItem    `json:"sNssaiUpfInfos" yaml:"sNssaiUpfInfos,omitempty" valid:"optional"`
	InterfaceUpfInfoList []InterfaceUpfInfoItem `json:"interfaces" yaml:"interfaces,omitempty" valid:"optional"`
	ULMBRLimit           string                 `json:"ulMbrLimit" yaml:"ulMbrLimit,omitempty" valid:"optional"`
	DLMBRLimit           string                 `json:"dlMbrLimit" yaml:"dlMbrLimit,omitempty" valid:"optional"`
}

func (u *UPNode) validate() (bool, error) {
	govalidator.TagMap["upNodeType"] = govalidator.Validator(func(str string) bool {
		return str == "AN" || str == "UPF"
	})

	for _, snssaiInfo := range u.SNssaiInfos {
		if result, err := snssaiInfo.validate(); err != nil {
			return result, err
		}
	}

	for _, interfaceUpfInfo := range u.InterfaceUpfInfoList {
		if result, err := interfaceUpfInfo.validate(); err != nil {
			return result, err
		}
	}
	result, err := govalidator.ValidateStruct(u)
	return result, appendInvalid(err)
}

type InterfaceUpfInfoItem struct {
	InterfaceType   models.UpInterfaceType `json:"interfaceType" yaml:"interfaceType" valid:"required"`
	Endpoints       []string               `json:"endpoints" yaml:"endpoints" valid:"required"`
	NetworkInstance string                 `json:"networkInstance" yaml:"networkInstance" valid:"required"`
}

func (i *InterfaceUpfInfoItem) validate() (bool, error) {
	interfaceType := i.InterfaceType
	if result := (interfaceType == "N3" || interfaceType == "N9"); !result {
		err := errors.New("Invalid interfaceType: " + string(interfaceType) + ", should be N3 or N9.")
		return false, err
	}

	for _, endpoint := range i.Endpoints {
		if result := govalidator.IsHost(endpoint); !result {
			err := errors.New("Invalid endpoint:" + endpoint + ", should be IPv4.")
			return false, err
		}
	}

	result, err := govalidator.ValidateStruct(i)
	return result, appendInvalid(err)
}

type SnssaiUpfInfoItem struct {
	SNssai         *models.Snssai   `json:"sNssai" yaml:"sNssai" valid:"required"`
	DnnUpfInfoList []DnnUpfInfoItem `json:"dnnUpfInfoList" yaml:"dnnUpfInfoList" valid:"required"`
}

func (s *SnssaiUpfInfoItem) validate() (bool, error) {
	if s.SNssai != nil {
		if result := (s.SNssai.Sst >= 0 && s.SNssai.Sst <= 255); !result {
			err := errors.New("Invalid sNssai.Sst: " + strconv.Itoa(int(s.SNssai.Sst)) + ", should be in range 0~255.")
			return false, err
		}

		if result := govalidator.StringMatches(s.SNssai.Sd, "^[0-9A-Fa-f]{6}$"); !result {
			err := errors.New("Invalid sNssai.Sd: " + s.SNssai.Sd +
				", should be 3 bytes hex string and in range 000000~FFFFFF.")
			return false, err
		}
	}

	for _, dnnInfo := range s.DnnUpfInfoList {
		if result, err := dnnInfo.validate(); err != nil {
			return result, err
		}
	}

	result, err := govalidator.ValidateStruct(s)
	return result, appendInvalid(err)
}

type DnnUpfInfoItem struct {
	Dnn             string                  `json:"dnn" yaml:"dnn" valid:"required"`
	DnaiList        []string                `json:"dnaiList" yaml:"dnaiList" valid:"optional"`
	PduSessionTypes []models.PduSessionType `json:"pduSessionTypes" yaml:"pduSessionTypes" valid:"optional"`
	Pools           []UEIPPool              `json:"pools" yaml:"pools" valid:"optional"`
}

func (d *DnnUpfInfoItem) validate() (bool, error) {
	if result := len(d.Dnn); result == 0 {
		err := errors.New("Invalid DnnUpfInfoItem.dnn: " + d.Dnn + ", should not be empty.")
		return false, err
	}

	for _, pool := range d.Pools {
		if result, err := pool.validate(); err != nil {
			return result, err
		}
	}

	result, err := govalidator.ValidateStruct(d)
	return result, appendInvalid(err)
}

type UPLink struct {
	A string `json:"A" yaml:"A" valid:"required"`
	B string `json:"B" yaml:"B" valid:"required"`
}

func (u *UPLink) validate() (bool, error) {
	result, err := govalidator.ValidateStruct(u)
	return result, appendInvalid(err)
}

func appendInvalid(err error) error {
	var errs govalidator.Errors

	if err == nil {
		return nil
	}

	es := err.(govalidator.Errors).Errors()
	for _, e := range es {
		errs = append(errs, fmt.Errorf("invalid %w", e))
	}

	return error(errs)
}

type UEIPPool struct {
	Cidr string `yaml:"cidr" valid:"cidr,required"`
}

func (u *UEIPPool) validate() (bool, error) {
	govalidator.TagMap["cidr"] = govalidator.Validator(func(str string) bool {
		return govalidator.IsCIDR(str)
	})

	result, err := govalidator.ValidateStruct(u)
	return result, appendInvalid(err)
}

type SpecificPath struct {
	DestinationIP   string   `yaml:"dest,omitempty" valid:"cidr,required"`
	DestinationPort string   `yaml:"DestinationPort,omitempty" valid:"port,optional"`
	Path            []string `yaml:"path" valid:"required"`
}

func (p *SpecificPath) validate() (bool, error) {
	govalidator.TagMap["cidr"] = govalidator.Validator(func(str string) bool {
		return govalidator.IsCIDR(str)
	})

	for _, upf := range p.Path {
		if result := len(upf); result == 0 {
			err := errors.New("Invalid UPF: " + upf + ", should not be empty")
			return false, err
		}
	}

	result, err := govalidator.ValidateStruct(p)
	return result, appendInvalid(err)
}

type PlmnID struct {
	Mcc string `yaml:"mcc"`
	Mnc string `yaml:"mnc"`
}

func (p *PlmnID) validate() (bool, error) {
	mcc := p.Mcc
	if result := govalidator.StringMatches(mcc, "^[0-9]{3}$"); !result {
		err := fmt.Errorf("Invalid mcc: %s, should be a 3-digit number", mcc)
		return false, err
	}

	mnc := p.Mnc
	if result := govalidator.StringMatches(mnc, "^[0-9]{2,3}$"); !result {
		err := fmt.Errorf("Invalid mnc: %s, should be a 2 or 3-digit number", mnc)
		return false, err
	}
	return true, nil
}

func (c *Config) GetVersion() string {
	if c.Info != nil && c.Info.Version != "" {
		return c.Info.Version
	}
	return ""
}

func (r *RoutingConfig) GetVersion() string {
	if r.Info != nil && r.Info.Version != "" {
		return r.Info.Version
	}
	return ""
}
