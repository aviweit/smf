package context

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"reflect"
	"sort"
	"sync"

	"github.com/free5gc/openapi/models"
	"github.com/free5gc/pfcp/pfcpType"
	"github.com/free5gc/smf/internal/context/pool"
	"github.com/free5gc/smf/internal/logger"
	"github.com/free5gc/smf/pkg/factory"
)

// UserPlaneInformation store userplane topology
type UserPlaneInformation struct {
	Mu                        sync.RWMutex // protect UPF and topology structure
	UPNodes                   map[string]*UPNode
	UPFs                      map[string]*UPNode
	AccessNetwork             map[string]*UPNode
	UPFIPToName               map[string]string
	UPFsID                    map[string]string               // name to id
	UPFsIPtoID                map[string]string               // ip->id table, for speed optimization
	DefaultUserPlanePath      map[string][]*UPNode            // DNN to Default Path
	DefaultUserPlanePathToUPF map[string]map[string][]*UPNode // DNN and UPF to Default Path
}

type UPNodeType string

const (
	UPNODE_UPF UPNodeType = "UPF"
	UPNODE_AN  UPNodeType = "AN"
)

// UPNode represent the user plane node topology
type UPNode struct {
	Type   UPNodeType
	NodeID pfcpType.NodeID
	ANIP   net.IP
	Dnn    string
	Links  []*UPNode
	UPF    *UPF
}

// UPPath represent User Plane Sequence of this path
type UPPath []*UPNode

func AllocateUPFID() {
	UPFsID := smfContext.UserPlaneInformation.UPFsID
	UPFsIPtoID := smfContext.UserPlaneInformation.UPFsIPtoID

	for upfName, upfNode := range smfContext.UserPlaneInformation.UPFs {
		upfid := upfNode.UPF.UUID()
		upfip := upfNode.NodeID.ResolveNodeIdToIp().String()

		UPFsID[upfName] = upfid
		UPFsIPtoID[upfip] = upfid
	}
}

// NewUserPlaneInformation process the configuration then returns a new instance of UserPlaneInformation
func NewUserPlaneInformation(upTopology *factory.UserPlaneInformation) *UserPlaneInformation {
	nodePool := make(map[string]*UPNode)
	upfPool := make(map[string]*UPNode)
	anPool := make(map[string]*UPNode)
	upfIPMap := make(map[string]string)
	allUEIPPools := []*UeIPPool{}

	for name, node := range upTopology.UPNodes {
		upNode := new(UPNode)
		upNode.Type = UPNodeType(node.Type)
		switch upNode.Type {
		case UPNODE_AN:
			upNode.ANIP = net.ParseIP(node.ANIP)
			anPool[name] = upNode
		case UPNODE_UPF:
			// ParseIp() always return 16 bytes
			// so we can't use the length of return ip to separate IPv4 and IPv6
			// This is just a work around
			var ip net.IP
			if net.ParseIP(node.NodeID).To4() == nil {
				ip = net.ParseIP(node.NodeID)
			} else {
				ip = net.ParseIP(node.NodeID).To4()
			}

			switch len(ip) {
			case net.IPv4len:
				upNode.NodeID = pfcpType.NodeID{
					NodeIdType: pfcpType.NodeIdTypeIpv4Address,
					IP:         ip,
				}
			case net.IPv6len:
				upNode.NodeID = pfcpType.NodeID{
					NodeIdType: pfcpType.NodeIdTypeIpv6Address,
					IP:         ip,
				}
			default:
				upNode.NodeID = pfcpType.NodeID{
					NodeIdType: pfcpType.NodeIdTypeFqdn,
					FQDN:       node.NodeID,
				}
			}

			upNode.UPF = NewUPF(&upNode.NodeID, node.InterfaceUpfInfoList)
			snssaiInfos := make([]SnssaiUPFInfo, 0)
			for _, snssaiInfoConfig := range node.SNssaiInfos {
				snssaiInfo := SnssaiUPFInfo{
					SNssai: SNssai{
						Sst: snssaiInfoConfig.SNssai.Sst,
						Sd:  snssaiInfoConfig.SNssai.Sd,
					},
					DnnList: make([]DnnUPFInfoItem, 0),
				}

				for _, dnnInfoConfig := range snssaiInfoConfig.DnnUpfInfoList {
					ueIPPools := make([]*UeIPPool, 0)
					for _, pool := range dnnInfoConfig.Pools {
						ueIPPool := NewUEIPPool(&pool)
						if ueIPPool == nil {
							logger.InitLog.Fatalf("invalid pools value: %+v", pool)
						} else {
							ueIPPools = append(ueIPPools, ueIPPool)
							allUEIPPools = append(allUEIPPools, ueIPPool)
						}
					}
					snssaiInfo.DnnList = append(snssaiInfo.DnnList, DnnUPFInfoItem{
						Dnn:             dnnInfoConfig.Dnn,
						DnaiList:        dnnInfoConfig.DnaiList,
						PduSessionTypes: dnnInfoConfig.PduSessionTypes,
						UeIPPools:       ueIPPools,
					})
				}
				snssaiInfos = append(snssaiInfos, snssaiInfo)
			}
			upNode.UPF.SNssaiInfos = snssaiInfos
			upfPool[name] = upNode
		default:
			logger.InitLog.Warningf("invalid UPNodeType: %s\n", upNode.Type)
		}

		nodePool[name] = upNode

		ipStr := upNode.NodeID.ResolveNodeIdToIp().String()
		upfIPMap[ipStr] = name
	}

	if isOverlap(allUEIPPools) {
		logger.InitLog.Fatalf("overlap cidr value between UPFs")
	}

	for _, link := range upTopology.Links {
		nodeA := nodePool[link.A]
		nodeB := nodePool[link.B]
		if nodeA == nil || nodeB == nil {
			logger.InitLog.Warningf("One of link edges does not exist. UPLink [%s] <=> [%s] not establish\n", link.A, link.B)
			continue
		}
		if nodeInLink(nodeB, nodeA.Links) != -1 || nodeInLink(nodeA, nodeB.Links) != -1 {
			logger.InitLog.Warningf("One of link edges already exist. UPLink [%s] <=> [%s] not establish\n", link.A, link.B)
			continue
		}
		nodeA.Links = append(nodeA.Links, nodeB)
		nodeB.Links = append(nodeB.Links, nodeA)
	}

	userplaneInformation := &UserPlaneInformation{
		UPNodes:                   nodePool,
		UPFs:                      upfPool,
		AccessNetwork:             anPool,
		UPFIPToName:               upfIPMap,
		UPFsID:                    make(map[string]string),
		UPFsIPtoID:                make(map[string]string),
		DefaultUserPlanePath:      make(map[string][]*UPNode),
		DefaultUserPlanePathToUPF: make(map[string]map[string][]*UPNode),
	}

	return userplaneInformation
}

func (upi *UserPlaneInformation) UpNodesToConfiguration() map[string]factory.UPNode {
	nodes := make(map[string]factory.UPNode)
	for name, upNode := range upi.UPNodes {
		u := new(factory.UPNode)
		switch upNode.Type {
		case UPNODE_UPF:
			u.Type = "UPF"
		case UPNODE_AN:
			u.Type = "AN"
			u.ANIP = upNode.ANIP.String()
		default:
			u.Type = "Unknown"
		}
		nodeIDtoIp := upNode.NodeID.ResolveNodeIdToIp()
		if nodeIDtoIp != nil {
			u.NodeID = nodeIDtoIp.String()
		}
		if upNode.UPF != nil {
			if upNode.UPF.SNssaiInfos != nil {
				FsNssaiInfoList := make([]factory.SnssaiUpfInfoItem, 0)
				for _, sNssaiInfo := range upNode.UPF.SNssaiInfos {
					FDnnUpfInfoList := make([]factory.DnnUpfInfoItem, 0)
					for _, dnnInfo := range sNssaiInfo.DnnList {
						FUEIPPools := make([]factory.UEIPPool, 0)
						for _, pool := range dnnInfo.UeIPPools {
							FUEIPPools = append(FUEIPPools, factory.UEIPPool{
								Cidr: pool.ueSubNet.String(),
							})
						} // for pool
						FDnnUpfInfoList = append(FDnnUpfInfoList, factory.DnnUpfInfoItem{
							Dnn:   dnnInfo.Dnn,
							Pools: FUEIPPools,
						})
					} // for dnnInfo
					Fsnssai := factory.SnssaiUpfInfoItem{
						SNssai: &models.Snssai{
							Sst: sNssaiInfo.SNssai.Sst,
							Sd:  sNssaiInfo.SNssai.Sd,
						},
						DnnUpfInfoList: FDnnUpfInfoList,
					}
					FsNssaiInfoList = append(FsNssaiInfoList, Fsnssai)
				} // for sNssaiInfo
				u.SNssaiInfos = FsNssaiInfoList
			} // if UPF.SNssaiInfos
			FNxList := make([]factory.InterfaceUpfInfoItem, 0)
			for _, iface := range upNode.UPF.N3Interfaces {
				endpoints := make([]string, 0)
				// upf.go L90
				if iface.EndpointFQDN != "" {
					endpoints = append(endpoints, iface.EndpointFQDN)
				}
				for _, eIP := range iface.IPv4EndPointAddresses {
					endpoints = append(endpoints, eIP.String())
				}
				FNxList = append(FNxList, factory.InterfaceUpfInfoItem{
					InterfaceType:   models.UpInterfaceType_N3,
					Endpoints:       endpoints,
					NetworkInstance: iface.NetworkInstance,
				})
			} // for N3Interfaces

			for _, iface := range upNode.UPF.N9Interfaces {
				endpoints := make([]string, 0)
				// upf.go L90
				if iface.EndpointFQDN != "" {
					endpoints = append(endpoints, iface.EndpointFQDN)
				}
				for _, eIP := range iface.IPv4EndPointAddresses {
					endpoints = append(endpoints, eIP.String())
				}
				FNxList = append(FNxList, factory.InterfaceUpfInfoItem{
					InterfaceType:   models.UpInterfaceType_N9,
					Endpoints:       endpoints,
					NetworkInstance: iface.NetworkInstance,
				})
			} // N9Interfaces
			u.InterfaceUpfInfoList = FNxList
		}
		nodes[name] = *u
	}

	return nodes
}

func (upi *UserPlaneInformation) LinksToConfiguration() []factory.UPLink {
	links := make([]factory.UPLink, 0)
	source, err := upi.selectUPPathSource()
	if err != nil {
		logger.InitLog.Errorf("AN Node not found\n")
	} else {
		visited := make(map[*UPNode]bool)
		queue := make([]*UPNode, 0)
		queue = append(queue, source)
		for {
			node := queue[0]
			queue = queue[1:]
			visited[node] = true
			for _, link := range node.Links {
				if !visited[link] {
					queue = append(queue, link)
					nodeIpStr := node.NodeID.ResolveNodeIdToIp().String()
					ipStr := link.NodeID.ResolveNodeIdToIp().String()
					linkA := upi.UPFIPToName[nodeIpStr]
					linkB := upi.UPFIPToName[ipStr]
					links = append(links, factory.UPLink{
						A: linkA,
						B: linkB,
					})
				}
			}
			if len(queue) == 0 {
				break
			}
		}
	}
	return links
}

func (upi *UserPlaneInformation) UpNodesFromConfiguration(upTopology *factory.UserPlaneInformation) {
	for name, node := range upTopology.UPNodes {
		if _, ok := upi.UPNodes[name]; ok {
			logger.InitLog.Warningf("Node [%s] already exists in SMF.\n", name)
			continue
		}
		upNode := new(UPNode)
		upNode.Type = UPNodeType(node.Type)
		switch upNode.Type {
		case UPNODE_UPF:
			// ParseIp() always return 16 bytes
			// so we can't use the length of return ip to separate IPv4 and IPv6
			// This is just a work around
			var ip net.IP
			if net.ParseIP(node.NodeID).To4() == nil {
				ip = net.ParseIP(node.NodeID)
			} else {
				ip = net.ParseIP(node.NodeID).To4()
			}

			switch len(ip) {
			case net.IPv4len:
				upNode.NodeID = pfcpType.NodeID{
					NodeIdType: pfcpType.NodeIdTypeIpv4Address,
					IP:         ip,
				}
			case net.IPv6len:
				upNode.NodeID = pfcpType.NodeID{
					NodeIdType: pfcpType.NodeIdTypeIpv6Address,
					IP:         ip,
				}
			default:
				upNode.NodeID = pfcpType.NodeID{
					NodeIdType: pfcpType.NodeIdTypeFqdn,
					FQDN:       node.NodeID,
				}
			}

			upNode.UPF = NewUPF(&upNode.NodeID, node.InterfaceUpfInfoList)
			snssaiInfos := make([]SnssaiUPFInfo, 0)
			for _, snssaiInfoConfig := range node.SNssaiInfos {
				snssaiInfo := SnssaiUPFInfo{
					SNssai: SNssai{
						Sst: snssaiInfoConfig.SNssai.Sst,
						Sd:  snssaiInfoConfig.SNssai.Sd,
					},
					DnnList: make([]DnnUPFInfoItem, 0),
				}

				for _, dnnInfoConfig := range snssaiInfoConfig.DnnUpfInfoList {
					ueIPPools := make([]*UeIPPool, 0)
					for _, pool := range dnnInfoConfig.Pools {
						ueIPPool := NewUEIPPool(&pool)
						if ueIPPool == nil {
							logger.InitLog.Fatalf("invalid pools value: %+v", pool)
						} else {
							ueIPPools = append(ueIPPools, ueIPPool)
						}
					}
					snssaiInfo.DnnList = append(snssaiInfo.DnnList, DnnUPFInfoItem{
						Dnn:             dnnInfoConfig.Dnn,
						DnaiList:        dnnInfoConfig.DnaiList,
						PduSessionTypes: dnnInfoConfig.PduSessionTypes,
						UeIPPools:       ueIPPools,
					})
				}
				snssaiInfos = append(snssaiInfos, snssaiInfo)
			}
			upNode.UPF.SNssaiInfos = snssaiInfos
			upi.UPFs[name] = upNode

			// AllocateUPFID
			upfid := upNode.UPF.UUID()
			upfip := upNode.NodeID.ResolveNodeIdToIp().String()
			upi.UPFsID[name] = upfid
			upi.UPFsIPtoID[upfip] = upfid

		case UPNODE_AN:
			upNode.ANIP = net.ParseIP(node.ANIP)
			upi.AccessNetwork[name] = upNode
		default:
			logger.InitLog.Warningf("invalid UPNodeType: %s\n", upNode.Type)
		}

		upi.UPNodes[name] = upNode

		ipStr := upNode.NodeID.ResolveNodeIdToIp().String()
		upi.UPFIPToName[ipStr] = name
	}

	// overlap UE IP pool validation
	allUEIPPools := []*UeIPPool{}
	for _, upf := range upi.UPFs {
		for _, snssaiInfo := range upf.UPF.SNssaiInfos {
			for _, dnn := range snssaiInfo.DnnList {
				allUEIPPools = append(allUEIPPools, dnn.UeIPPools...)
			}
		}
	}
	if isOverlap(allUEIPPools) {
		logger.InitLog.Fatalf("overlap cidr value between UPFs")
	}
}

func (upi *UserPlaneInformation) LinksFromConfiguration(upTopology *factory.UserPlaneInformation) {
	for _, link := range upTopology.Links {
		nodeA := upi.UPNodes[link.A]
		nodeB := upi.UPNodes[link.B]
		if nodeA == nil || nodeB == nil {
			logger.InitLog.Warningf("One of link edges does not exist. UPLink [%s] <=> [%s] not establish\n", link.A, link.B)
			continue
		}
		if nodeInLink(nodeB, nodeA.Links) != -1 || nodeInLink(nodeA, nodeB.Links) != -1 {
			logger.InitLog.Warningf("One of link edges already exist. UPLink [%s] <=> [%s] not establish\n", link.A, link.B)
			continue
		}
		nodeA.Links = append(nodeA.Links, nodeB)
		nodeB.Links = append(nodeB.Links, nodeA)
	}
}

func (upi *UserPlaneInformation) UpNodeDelete(upNodeName string) {
	upNode, ok := upi.UPNodes[upNodeName]
	if ok {
		logger.InitLog.Infof("UPNode [%s] found. Deleting it.\n", upNodeName)
		if upNode.Type == UPNODE_UPF {
			logger.InitLog.Tracef("Delete UPF [%s] from its NodeID.\n", upNodeName)
			RemoveUPFNodeByNodeID(upNode.UPF.NodeID)
			if _, ok = upi.UPFs[upNodeName]; ok {
				logger.InitLog.Tracef("Delete UPF [%s] from upi.UPFs.\n", upNodeName)
				delete(upi.UPFs, upNodeName)
			}
			for selectionStr, destMap := range upi.DefaultUserPlanePathToUPF {
				for destIp, path := range destMap {
					if nodeInPath(upNode, path) != -1 {
						logger.InitLog.Infof("Invalidate cache entry: DefaultUserPlanePathToUPF[%s][%s].\n", selectionStr, destIp)
						delete(upi.DefaultUserPlanePathToUPF[selectionStr], destIp)
					}
				}
			}
		}
		if upNode.Type == UPNODE_AN {
			logger.InitLog.Tracef("Delete AN [%s] from upi.AccessNetwork.\n", upNodeName)
			delete(upi.AccessNetwork, upNodeName)
		}
		logger.InitLog.Tracef("Delete UPNode [%s] from upi.UPNodes.\n", upNodeName)
		delete(upi.UPNodes, upNodeName)

		// update links
		for name, n := range upi.UPNodes {
			if index := nodeInLink(upNode, n.Links); index != -1 {
				logger.InitLog.Infof("Delete UPLink [%s] <=> [%s].\n", name, upNodeName)
				n.Links = removeNodeFromLink(n.Links, index)
			}
		}
	}
}

func NewUEIPPool(factoryPool *factory.UEIPPool) *UeIPPool {
	_, ipNet, err := net.ParseCIDR(factoryPool.Cidr)
	if err != nil {
		logger.InitLog.Errorln(err)
		return nil
	}

	minAddr, maxAddr, err := calcAddrRange(ipNet)
	if err != nil {
		logger.InitLog.Errorln(err)
		return nil
	}

	newPool, err := pool.NewLazyReusePool(int(minAddr), int(maxAddr))
	if err != nil {
		logger.InitLog.Errorln(err)
		return nil
	}

	ueIPPool := &UeIPPool{
		ueSubNet: ipNet,
		pool:     newPool,
	}
	return ueIPPool
}

func calcAddrRange(ipNet *net.IPNet) (minAddr, maxAddr uint32, err error) {
	maskVal := binary.BigEndian.Uint32(ipNet.Mask)
	baseIPVal := binary.BigEndian.Uint32(ipNet.IP)
	if maskVal == math.MaxUint32 {
		return baseIPVal, baseIPVal, nil
	}
	minAddr = (baseIPVal & maskVal) + 1  // 0 is network address
	maxAddr = (baseIPVal | ^maskVal) - 1 // all 1 is broadcast address
	if minAddr > maxAddr {
		return minAddr, maxAddr, errors.New("Mask is invalid.")
	}
	return minAddr, maxAddr, nil
}

func isOverlap(pools []*UeIPPool) bool {
	if len(pools) < 2 {
		// no need to check
		return false
	}
	for i := 0; i < len(pools)-1; i++ {
		for j := i + 1; j < len(pools); j++ {
			if pools[i].pool.IsJoint(pools[j].pool) {
				return true
			}
		}
	}
	return false
}

func nodeInPath(upNode *UPNode, path []*UPNode) int {
	for i, u := range path {
		if u == upNode {
			return i
		}
	}
	return -1
}

func removeNodeFromLink(links []*UPNode, index int) []*UPNode {
	links[index] = links[len(links)-1]
	return links[:len(links)-1]
}

func nodeInLink(upNode *UPNode, links []*UPNode) int {
	for i, n := range links {
		if n == upNode {
			return i
		}
	}
	return -1
}

func (upi *UserPlaneInformation) GetUPFNameByIp(ip string) string {
	return upi.UPFIPToName[ip]
}

func (upi *UserPlaneInformation) GetUPFNodeIDByName(name string) pfcpType.NodeID {
	return upi.UPFs[name].NodeID
}

func (upi *UserPlaneInformation) GetUPFNodeByIP(ip string) *UPNode {
	upfName := upi.GetUPFNameByIp(ip)
	return upi.UPFs[upfName]
}

func (upi *UserPlaneInformation) GetUPFIDByIP(ip string) string {
	return upi.UPFsIPtoID[ip]
}

func (upi *UserPlaneInformation) GetDefaultUserPlanePathByDNN(selection *UPFSelectionParams) (path UPPath) {
	path, pathExist := upi.DefaultUserPlanePath[selection.String()]
	logger.CtxLog.Traceln("In GetDefaultUserPlanePathByDNN")
	logger.CtxLog.Traceln("selection: ", selection.String())
	if pathExist {
		return
	} else {
		pathExist = upi.GenerateDefaultPath(selection)
		if pathExist {
			return upi.DefaultUserPlanePath[selection.String()]
		}
	}
	return nil
}

func (upi *UserPlaneInformation) GetDefaultUserPlanePathByDNNAndUPF(
	selection *UPFSelectionParams,
	upf *UPNode,
) (path UPPath) {
	nodeID := upf.NodeID.ResolveNodeIdToIp().String()

	if upi.DefaultUserPlanePathToUPF[selection.String()] != nil {
		path, pathExist := upi.DefaultUserPlanePathToUPF[selection.String()][nodeID]
		logger.CtxLog.Traceln("In GetDefaultUserPlanePathByDNNAndUPF")
		logger.CtxLog.Traceln("selection: ", selection.String())
		logger.CtxLog.Traceln("pathExist: ", pathExist)
		if pathExist {
			return path
		}
	}
	if pathExist := upi.GenerateDefaultPathToUPF(selection, upf); pathExist {
		return upi.DefaultUserPlanePathToUPF[selection.String()][nodeID]
	}
	return nil
}

func (upi *UserPlaneInformation) ExistDefaultPath(dnn string) bool {
	_, exist := upi.DefaultUserPlanePath[dnn]
	return exist
}

func GenerateDataPath(upPath UPPath, smContext *SMContext) *DataPath {
	if len(upPath) < 1 {
		logger.CtxLog.Errorf("Invalid data path")
		return nil
	}
	lowerBound := 0
	upperBound := len(upPath) - 1
	var root *DataPathNode
	var curDataPathNode *DataPathNode
	var prevDataPathNode *DataPathNode

	for idx, upNode := range upPath {
		curDataPathNode = NewDataPathNode()
		curDataPathNode.UPF = upNode.UPF

		if idx == lowerBound {
			root = curDataPathNode
			root.AddPrev(nil)
		}
		if idx == upperBound {
			curDataPathNode.AddNext(nil)
		}
		if prevDataPathNode != nil {
			prevDataPathNode.AddNext(curDataPathNode)
			curDataPathNode.AddPrev(prevDataPathNode)
		}
		prevDataPathNode = curDataPathNode
	}

	dataPath := &DataPath{
		Destination: Destination{
			DestinationIP:   "",
			DestinationPort: "",
			Url:             "",
		},
		FirstDPNode: root,
	}
	return dataPath
}

func (upi *UserPlaneInformation) GenerateDefaultPath(selection *UPFSelectionParams) bool {
	var source *UPNode
	var destinations []*UPNode

	for _, node := range upi.AccessNetwork {
		if node.Type == UPNODE_AN {
			source = node
			break
		}
	}

	if source == nil {
		logger.CtxLog.Errorf("There is no AN Node in config file!")
		return false
	}

	destinations = upi.selectMatchUPF(selection)

	if len(destinations) == 0 {
		logger.CtxLog.Errorf("Can't find UPF with DNN[%s] S-NSSAI[sst: %d sd: %s] DNAI[%s]\n", selection.Dnn,
			selection.SNssai.Sst, selection.SNssai.Sd, selection.Dnai)
		return false
	} else {
		logger.CtxLog.Tracef("Find UPF with DNN[%s] S-NSSAI[sst: %d sd: %s] DNAI[%s]\n", selection.Dnn,
			selection.SNssai.Sst, selection.SNssai.Sd, selection.Dnai)
	}

	// Run DFS
	visited := make(map[*UPNode]bool)

	for _, upNode := range upi.UPNodes {
		visited[upNode] = false
	}

	path, pathExist := getPathBetween(source, destinations[0], visited, selection)

	if pathExist {
		if path[0].Type == UPNODE_AN {
			path = path[1:]
		}
		upi.DefaultUserPlanePath[selection.String()] = path
	}

	return pathExist
}

func (upi *UserPlaneInformation) GenerateDefaultPathToUPF(selection *UPFSelectionParams, destination *UPNode) bool {
	var source *UPNode

	for _, node := range upi.AccessNetwork {
		if node.Type == UPNODE_AN {
			source = node
			break
		}
	}

	if source == nil {
		logger.CtxLog.Errorf("There is no AN Node in config file!")
		return false
	}

	// Run DFS
	visited := make(map[*UPNode]bool)

	for _, upNode := range upi.UPNodes {
		visited[upNode] = false
	}

	path, pathExist := getPathBetween(source, destination, visited, selection)

	if pathExist {
		if path[0].Type == UPNODE_AN {
			path = path[1:]
		}
		if upi.DefaultUserPlanePathToUPF[selection.String()] == nil {
			upi.DefaultUserPlanePathToUPF[selection.String()] = make(map[string][]*UPNode)
		}
		upi.DefaultUserPlanePathToUPF[selection.String()][destination.NodeID.ResolveNodeIdToIp().String()] = path
	}

	return pathExist
}

func (upi *UserPlaneInformation) selectMatchUPF(selection *UPFSelectionParams) []*UPNode {
	upList := make([]*UPNode, 0)

	for _, upNode := range upi.UPFs {
		for _, snssaiInfo := range upNode.UPF.SNssaiInfos {
			currentSnssai := &snssaiInfo.SNssai
			targetSnssai := selection.SNssai

			if currentSnssai.Equal(targetSnssai) {
				for _, dnnInfo := range snssaiInfo.DnnList {
					if dnnInfo.Dnn == selection.Dnn && dnnInfo.ContainsDNAI(selection.Dnai) {
						upList = append(upList, upNode)
						break
					}
				}
			}
		}
	}
	return upList
}

func getPathBetween(
	cur *UPNode, dest *UPNode, visited map[*UPNode]bool,
	selection *UPFSelectionParams,
) (path []*UPNode, pathExist bool) {
	visited[cur] = true

	if reflect.DeepEqual(*cur, *dest) {
		path = make([]*UPNode, 0)
		path = append(path, cur)
		pathExist = true
		return
	}

	selectedSNssai := selection.SNssai

	for _, node := range cur.Links {
		if !visited[node] {
			if !node.UPF.isSupportSnssai(selectedSNssai) {
				visited[node] = true
				continue
			}

			path_tail, path_exist := getPathBetween(node, dest, visited, selection)

			if path_exist {
				path = make([]*UPNode, 0)
				path = append(path, cur)

				path = append(path, path_tail...)
				pathExist = true

				return
			}
		}
	}

	return nil, false
}

func (upi *UserPlaneInformation) selectAnchorUPF(source *UPNode, selection *UPFSelectionParams) []*UPNode {
	upList := make([]*UPNode, 0)
	visited := make(map[*UPNode]bool)
	queue := make([]*UPNode, 0)
	targetSnssai := selection.SNssai

	queue = append(queue, source)
	for {
		node := queue[0]
		queue = queue[1:]
		findNewNode := false
		visited[node] = true
		for _, link := range node.Links {
			if !visited[link] {
				for _, snssaiInfo := range link.UPF.SNssaiInfos {
					currentSnssai := &snssaiInfo.SNssai
					if currentSnssai.Equal(targetSnssai) {
						for _, dnnInfo := range snssaiInfo.DnnList {
							if dnnInfo.Dnn == selection.Dnn && dnnInfo.ContainsDNAI(selection.Dnai) {
								queue = append(queue, link)
								findNewNode = true
								break
							}
						}
					}
				}
			}
		}
		if !findNewNode && node.Type == UPNODE_UPF {
			upList = append(upList, node)
		}
		if len(queue) == 0 {
			break
		}
	}
	return upList
}

func (upi *UserPlaneInformation) sortUPFListByName(upfList []*UPNode) []*UPNode {
	keys := make([]string, 0, len(upi.UPFs))
	for k := range upi.UPFs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sortedUpList := make([]*UPNode, 0)
	for _, name := range keys {
		for _, node := range upfList {
			if name == upi.GetUPFNameByIp(node.NodeID.ResolveNodeIdToIp().String()) {
				sortedUpList = append(sortedUpList, node)
			}
		}
	}
	return sortedUpList
}

func (upi *UserPlaneInformation) selectUPPathSource() (*UPNode, error) {
	// if multiple gNBs exist, select one according to some criterion
	for _, node := range upi.AccessNetwork {
		if node.Type == UPNODE_AN {
			return node, nil
		}
	}
	return nil, errors.New("AN Node not found")
}

func (upi *UserPlaneInformation) SelectUPFAndAllocUEIP(selection *UPFSelectionParams) (*UPNode, net.IP) {
	source, err := upi.selectUPPathSource()
	if err != nil {
		return nil, nil
	}
	UPFList := upi.selectAnchorUPF(source, selection)
	listLength := len(UPFList)
	if listLength == 0 {
		logger.CtxLog.Warnf("Can't find UPF with DNN[%s] S-NSSAI[sst: %d sd: %s] DNAI[%s]\n", selection.Dnn,
			selection.SNssai.Sst, selection.SNssai.Sd, selection.Dnai)
		return nil, nil
	}
	UPFList = upi.sortUPFListByName(UPFList)
	sortedUPFList := createUPFListForSelection(UPFList)
	for _, upf := range sortedUPFList {
		logger.CtxLog.Debugf("check start UPF: %s",
			upi.GetUPFNameByIp(upf.NodeID.ResolveNodeIdToIp().String()))
		if upf.UPF.UPFStatus != AssociatedSetUpSuccess {
			logger.CtxLog.Infof("PFCP Association not yet Established with: %s",
				upi.GetUPFNameByIp(upf.NodeID.ResolveNodeIdToIp().String()))
			continue
		}
		pools := getUEIPPool(upf, selection)
		if len(pools) == 0 {
			continue
		}
		sortedPoolList := createPoolListForSelection(pools)
		for _, pool := range sortedPoolList {
			logger.CtxLog.Debugf("check start UEIPPool(%+v)", pool.ueSubNet)
			addr := pool.allocate()
			if addr != nil {
				logger.CtxLog.Infof("Selected UPF: %s",
					upi.GetUPFNameByIp(upf.NodeID.ResolveNodeIdToIp().String()))
				return upf, addr
			}
			// if all addresses in pool are used, search next pool
			logger.CtxLog.Debug("check next pool")
		}
		// if all addresses in UPF are used, search next UPF
		logger.CtxLog.Debug("check next upf")
	}
	// checked all UPFs
	logger.CtxLog.Warnf("UE IP pool exhausted for DNN[%s] S-NSSAI[sst: %d sd: %s] DNAI[%s]\n", selection.Dnn,
		selection.SNssai.Sst, selection.SNssai.Sd, selection.Dnai)
	return nil, nil
}

func createUPFListForSelection(inputList []*UPNode) (outputList []*UPNode) {
	offset := rand.Intn(len(inputList))
	return append(inputList[offset:], inputList[:offset]...)
}

func createPoolListForSelection(inputList []*UeIPPool) (outputList []*UeIPPool) {
	offset := rand.Intn(len(inputList))
	return append(inputList[offset:], inputList[:offset]...)
}

func getUEIPPool(upNode *UPNode, selection *UPFSelectionParams) []*UeIPPool {
	for _, snssaiInfo := range upNode.UPF.SNssaiInfos {
		currentSnssai := &snssaiInfo.SNssai
		targetSnssai := selection.SNssai

		if currentSnssai.Equal(targetSnssai) {
			for _, dnnInfo := range snssaiInfo.DnnList {
				if dnnInfo.Dnn == selection.Dnn && dnnInfo.ContainsDNAI(selection.Dnai) {
					return dnnInfo.UeIPPools
				}
			}
		}
	}
	return nil
}

func (ueIPPool *UeIPPool) allocate() net.IP {
	allocVal, res := ueIPPool.pool.Allocate()
	if !res {
		logger.CtxLog.Warnf("Pool is empty: %+v", ueIPPool.ueSubNet)
		return nil
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(allocVal))
	logger.CtxLog.Infof("Allocated UE IP address: %v", net.IPv4(buf[0], buf[1], buf[2], buf[3]))
	return buf
}

func (upi *UserPlaneInformation) ReleaseUEIP(upf *UPNode, addr net.IP) {
	pool := findPoolByAddr(upf, addr)
	if pool == nil {
		// nothing to do
		logger.CtxLog.Warnf("Fail to release UE IP address: %v to UPF: %s",
			upi.GetUPFNameByIp(upf.NodeID.ResolveNodeIdToIp().String()), addr)
		return
	}
	pool.release(addr)
}

func findPoolByAddr(upf *UPNode, addr net.IP) *UeIPPool {
	for _, snssaiInfo := range upf.UPF.SNssaiInfos {
		for _, dnnInfo := range snssaiInfo.DnnList {
			for _, pool := range dnnInfo.UeIPPools {
				if pool.ueSubNet.Contains(addr) {
					return pool
				}
			}
		}
	}
	return nil
}

func (ueIPPool *UeIPPool) release(addr net.IP) {
	addrVal := binary.BigEndian.Uint32(addr)
	res := ueIPPool.pool.Free(int(addrVal))
	if !res {
		logger.CtxLog.Warnf("failed to release UE Address: %s", addr)
	}
	logger.CtxLog.Debug(ueIPPool.dump())
}

func (ueIPPool *UeIPPool) dump() string {
	str := "["
	elements := ueIPPool.pool.Dump()
	for index, element := range elements {
		var firstAddr net.IP
		var lastAddr net.IP
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(element[0]))
		firstAddr = buf
		buf = make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(element[1]))
		lastAddr = buf
		if index > 0 {
			str += ("->")
		}
		str += fmt.Sprintf("{%s - %s}", firstAddr.String(), lastAddr.String())
	}
	str += ("]")
	return str
}
