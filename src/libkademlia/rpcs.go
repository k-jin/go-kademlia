package libkademlia

// Contains definitions mirroring the Kademlia spec. You will need to stick
// strictly to these to be compatible with the reference implementation and
// other groups' code.

import (
	"net"
	"fmt"
)

type KademliaRPC struct {
	kademlia *Kademlia
}

// Host identification.
type Contact struct {
	NodeID ID
	Host   net.IP
	Port   uint16
}

type KBucket struct {
	Contacts []Contact
}

///////////////////////////////////////////////////////////////////////////////
// PING
///////////////////////////////////////////////////////////////////////////////
type PingMessage struct {
	Sender Contact
	MsgID  ID
}

type PongMessage struct {
	MsgID  ID
	Sender Contact
}

func (k *KademliaRPC) Ping(ping PingMessage, pong *PongMessage) error {
	pong.MsgID = CopyID(ping.MsgID)
	pong.Sender = k.kademlia.SelfContact

	err := k.kademlia.Update(&ping.Sender)
	if err != nil {
		return &CommandFailed{
		"Update failed in Ping" + fmt.Sprintf("ping message: %v \n pong message: %v", ping, pong)}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// STORE
///////////////////////////////////////////////////////////////////////////////
type StoreRequest struct {
	Sender Contact
	MsgID  ID
	Key    ID
	Value  []byte
}

type StoreResult struct {
	MsgID ID
	Err   error
}

func (k *KademliaRPC) Store(req StoreRequest, res *StoreResult) error {
	res.MsgID = req.MsgID
	err := k.kademlia.Update(&req.Sender)
	if err != nil {
		fmt.Printf("Store broke ", err)
		res.Err = err
		return err
	}
	addReq := VTableMsg{"add", req.Key, req.Value, nil}
	k.kademlia.VTableReqChan <- addReq
	addRes := <- k.kademlia.VTableResChan
	if addRes.Err != nil {
		res.Err = addRes.Err
		return addRes.Err
	}

	res.Err = nil
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// FIND_NODE
///////////////////////////////////////////////////////////////////////////////
type FindNodeRequest struct {
	Sender Contact
	MsgID  ID
	NodeID ID
}

type FindNodeResult struct {
	MsgID ID
	Nodes []Contact
	Err   error
}

func (k *KademliaRPC) FindNode(req FindNodeRequest, res *FindNodeResult) error {
	err := k.kademlia.Update(&req.Sender)
	if err != nil {
		return &CommandFailed{
			"Update failed in FindNode " }
	}
	res.MsgID = CopyID(req.MsgID)

	nodes, err :=  k.kademlia.NearestHelper(req.NodeID)
	if err!=nil {
		return &CommandFailed{ " FindNode failed"}
	}
	res.Nodes = nodes
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// FIND_VALUE
///////////////////////////////////////////////////////////////////////////////
type FindValueRequest struct {
	Sender Contact
	MsgID  ID
	Key    ID
}

// If Value is nil, it should be ignored, and Nodes means the same as in a
// FindNodeResult.
type FindValueResult struct {
	MsgID ID
	Value []byte
	Nodes []Contact
	Err   error
}

func (k *KademliaRPC) FindValue(req FindValueRequest, res *FindValueResult) error {
	// the key B is the the search key 

	// fmt.Println("In FindValue RPC")
	err := k.kademlia.Update(&req.Sender)
	if err != nil {
		res.Nodes = nil
		res.Value = nil
		res.Err = err
		return &CommandFailed{
			"Update failed in FindValue " }
	}
	res.MsgID = CopyID(req.MsgID)

	//req.Key is the target ID
	getReq := VTableMsg{"get", req.Key, nil, nil}
	k.kademlia.VTableReqChan <- getReq
	getRes := <- k.kademlia.VTableResChan
	if getRes.Err != nil {
		res.Nodes = nil
		res.Value = nil
		res.Err = getRes.Err
		return &CommandFailed{"Get Request failed in FindValue"}
	}
	value := getRes.Value
	// value:=k.kademlia.ValueTable[req.Key]
	if value == nil {
			nodes, err :=  k.kademlia.NearestHelper(req.Key)
			if err!=nil {
				res.Nodes = nil
				res.Value = nil
				res.Err = err
				return &CommandFailed{ " FindNode failed"}
			}
			res.Nodes = nodes
			res.Value = nil
		 
	} else {
		res.Value = value
		res.Nodes = nil
	}


	return nil
	
	
}

// For Project 3

type GetVDORequest struct {
	Sender Contact
	VdoID  ID
	MsgID  ID
}

type GetVDOResult struct {
	MsgID ID
	VDO   VanashingDataObject
}

func (k *KademliaRPC) GetVDO(req GetVDORequest, res *GetVDOResult) error {
	// TODO: Implement.
	return nil
}
