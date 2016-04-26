package libkademlia

// Contains the core kademlia type. In addition to core state, this type serves
// as a receiver for the RPC methods, which is required by that package.

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
)

const (
	alpha = 3
	b     = 8 * IDBytes
	k     = 20
)

// Kademlia type. You can put whatever state you need in this.
type Kademlia struct {
	NodeID      ID
	SelfContact Contact
	KBucketsReqChan chan KBucketsMsg
	KBucketsResChan chan KBucketsMsg
	VTableReqChan	chan VTableMsg
	VTableResChan	chan VTableMsg
}

type KBucketsMsg struct {
	Request 	string
	Key 		int
	Value 		KBucket
	Err 		error
}


type VTableMsg struct {
	Request 	string
	Key 		ID
	Value 		[]byte
	Err 		error
}

func (k *Kademlia) KBucketsManager() {
	KBuckets := make(map[int]KBucket)
	for {
		req := <- k.KBucketsReqChan
		var res KBucketsMsg
		res = req
		res.Err = nil
		// if req.Key == nil {
		// 	res.Err = &CommandFailed{"No key provided"}
		// } else {
			switch req.Request {
			case "get":
				res.Value = KBuckets[req.Key]
			case "update":
				KBuckets[req.Key] = req.Value
			case "add":
				//TODO should we add error checking to see if Key already exists?
				KBuckets[req.Key] = req.Value
			case "delete":
				delete(KBuckets, res.Key)
			default:
				res.Err = &CommandFailed{"Invalid operation"}
			}
		// }
		
		k.KBucketsResChan <- res
	}
}

func (k *Kademlia) VTableManager() {
	ValueTable := make(map[ID][]byte)
	for {
		req := <- k.VTableReqChan
		var res VTableMsg
		res = req
		res.Err = nil
		// if req.Key == nil {
		// 	res.Err = &CommandFailed{"No key provided"}
		// } else {
			switch req.Request {
			case "get":
				res.Value = ValueTable[req.Key]
			case "update":
				ValueTable[req.Key] = req.Value
			case "add":
				//TODO should we add error checking to see if Key already exists?
				ValueTable[req.Key] = req.Value
			case "delete":
				delete(ValueTable, res.Key)
			default:
				res.Err = &CommandFailed{"Invalid operation"}
			}
		// }
		
		k.VTableResChan <- res
	}
}


func NewKademliaWithId(laddr string, nodeID ID) *Kademlia {
	k := new(Kademlia)
	k.NodeID = nodeID

	// TODO: Initialize other state here as you add functionality.
	
	k.KBucketsReqChan = make(chan KBucketsMsg)
	k.KBucketsResChan = make(chan KBucketsMsg)
	k.VTableReqChan = make(chan VTableMsg)
	k.VTableResChan = make(chan VTableMsg)
	// Set up RPC server
	// NOTE: KademliaRPC is just a wrapper around Kademlia. This type includes
	// the RPC functions.

	s := rpc.NewServer()
	s.Register(&KademliaRPC{k})
	hostname, port, err := net.SplitHostPort(laddr)
	if err != nil {
		return nil
	}
	s.HandleHTTP(rpc.DefaultRPCPath + port,
		rpc.DefaultDebugPath + port)
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		log.Fatal("Listen: ", err)
	}

	// Run RPC server forever.
	go http.Serve(l, nil)

	// Add self contact
	hostname, port, _ = net.SplitHostPort(l.Addr().String())
	port_int, _ := strconv.Atoi(port)
	ipAddrStrings, err := net.LookupHost(hostname)
	var host net.IP
	for i := 0; i < len(ipAddrStrings); i++ {
		host = net.ParseIP(ipAddrStrings[i])
		if host.To4() != nil {
			break
		}
	}
	k.SelfContact = Contact{k.NodeID, host, uint16(port_int)}
	go k.KBucketsManager()
	go k.VTableManager()
	return k
}

func NewKademlia(laddr string) *Kademlia {
	return NewKademliaWithId(laddr, NewRandomID())
}

func (k *Kademlia) Update(contact *Contact) error {

	if contact.NodeID == k.SelfContact.NodeID {
		return nil
	}
	bucket_id := contact.NodeID.Xor(k.SelfContact.NodeID).PrefixLen()
	getReq := KBucketsMsg{Request: "get", Key: bucket_id}
	k.KBucketsReqChan <- getReq
	getRes := <- k.KBucketsResChan
	if getRes.Err != nil {
		log.Println("Error in update KBucketsReqChan: ", getRes.Err)
		return getRes.Err
	}
	bucket := getRes.Value
	_, findErr := k.FindContact(contact.NodeID)
	if findErr != nil {
		// did not find contact
		if len(bucket.Contacts) < 20 {
			bucket.Contacts = append(bucket.Contacts, *contact)
		} else {
			oldContact := bucket.Contacts[0]
			_, err := k.DoPing(oldContact.Host, oldContact.Port)
			if err != nil{
				// node did not respond
				bucket.Contacts = bucket.Contacts[1:]
				bucket.Contacts = append(bucket.Contacts, *contact)
			} else {
				// node responded
				currContact := bucket.Contacts[0]
				bucket.Contacts = bucket.Contacts[1:]
				bucket.Contacts = append(bucket.Contacts, currContact)
			}
		}
	} else {
		// found contact
		for i, bucketContact := range bucket.Contacts {
			if contact.NodeID == bucketContact.NodeID {
				bucket.Contacts = append(bucket.Contacts[:i], bucket.Contacts[i+1:]...)
				bucket.Contacts = append(bucket.Contacts, bucketContact)
				break
			}
		}

	}
	updateReq := KBucketsMsg{"update", bucket_id, bucket, nil}
	k.KBucketsReqChan <- updateReq
	updateRes := <- k.KBucketsResChan
	if updateRes.Err != nil{
		return updateRes.Err
	}
	return nil

}

type ContactNotFoundError struct {
	id  ID
	msg string
}

func (e *ContactNotFoundError) Error() string {
	return fmt.Sprintf("%x %s", e.id, e.msg)
}

func (k *Kademlia) FindContact(nodeId ID) (*Contact, error) {
	// TODO: Search through contacts, find specified ID
	// Find contact with provided ID
	if nodeId == k.SelfContact.NodeID {
		return &k.SelfContact, nil
	}

	bucket_id := nodeId.Xor(k.SelfContact.NodeID).PrefixLen()
	getReq := KBucketsMsg{Request: "get", Key: bucket_id}
	k.KBucketsReqChan <- getReq
	getRes := <- k.KBucketsResChan
	if getRes.Err != nil {
		log.Println("Error in update KBucketsReqChan: ", getRes.Err)
		return nil, getRes.Err
	}
	bucket := getRes.Value
	for _, contact := range bucket.Contacts {
		if contact.NodeID == nodeId {
			return &contact, nil
		}
	}
	return nil, &ContactNotFoundError{nodeId, "Not found"}
}

type CommandFailed struct {
	msg string
}

func (e *CommandFailed) Error() string {
	return fmt.Sprintf("%s", e.msg)
}

// host - IP address of destination
// port - of destination
/*
	1. Send ping to destination
	2. Wait for pong
		a. If response
			1. Check if it exists, update if so
			2. Add if not full
			3. Drop if full
		b. Do nothing
*/
func (k *Kademlia) DoPing(host net.IP, port uint16) (*Contact, error) {
	// TODO: Implement
	address := fmt.Sprintf("%s:%v", host.String(), port)
	portStr := fmt.Sprintf("%v", port)
	client, err := rpc.DialHTTPPath("tcp", address, rpc.DefaultRPCPath + portStr)
	if err != nil {
		log.Printf("DialHTTPPath err", err)
		return nil, &CommandFailed{
		"Unable to ping " + fmt.Sprintf("%s:%v", host.String(), port)}
	}

	ping := PingMessage{k.SelfContact, NewRandomID()}
	var pong PongMessage
	err = client.Call("KademliaRPC.Ping", &ping, &pong)
	if err != nil {
		log.Printf("client.Call err", err)
		return nil, &CommandFailed{
		"Unable to ping " + fmt.Sprintf("%s:%v", host.String(), port)}
	} else {
		err = k.Update(&pong.Sender)
		if err != nil {
			log.Printf("Update err", err)
			return nil, &CommandFailed{
			"Update failed in DoPing: " + fmt.Sprintf("%s:%v", host.String(), port)}
		}
		return &pong.Sender, nil
	}
}




func (k *Kademlia) DoStore(contact *Contact, key ID, value []byte) error {
	// TODO: Implement
	address := fmt.Sprintf("%s:%v", contact.Host.String(), contact.Port)
	portStr := fmt.Sprintf("%v", contact.Port)

	client, err := rpc.DialHTTPPath("tcp", address, rpc.DefaultRPCPath + portStr)
	if err != nil {
		log.Printf("%v", err)
		return err
	} 
	storeReq := StoreRequest{k.SelfContact, NewRandomID(), key, value}
	var storeRes StoreResult
	err = client.Call("KademliaRPC.Store", &storeReq, &storeRes)
	if err != nil {
		return err
	}

	err = k.Update(contact) 
	if err !=  nil {
		log.Printf("Update err", err)
		return err
	}
	return nil

}

func (k *Kademlia) DoFindNode(contact *Contact, searchKey ID) ([]Contact, error) {
	// TODO: Implement

	host := contact.Host
	port := contact.Port
	address := fmt.Sprintf("%s:%v", host.String(), port)
	portStr := fmt.Sprintf("%v", port)
	client, err := rpc.DialHTTPPath("tcp", address, rpc.DefaultRPCPath + portStr)
	if err != nil {
		return nil, &CommandFailed{
		"Unable to ping " + fmt.Sprintf("%s:%v", host.String(), port)}
	}
	// log.Printf("Sending DoFindNode request\n")

	request := FindNodeRequest{k.SelfContact, NewRandomID(), searchKey}
	var result FindNodeResult
	err = client.Call("KademliaRPC.FindNode", &request, &result)
	if err != nil {
		log.Printf("Find Node Error", err)
		return nil, &CommandFailed{
		"Unable to Find Node " + fmt.Sprintf("%s:%v", host.String(), port)}
	} else {

		for _, node := range result.Nodes {
			err := k.Update(&node)
			if err !=nil {
				return nil, err
			}
		}
		return result.Nodes, nil
	}

}

func (k *Kademlia) DoFindValue(contact *Contact,
	searchKey ID) (value []byte, contacts []Contact, err error) {
	// TODO: Implement

	host:= contact.Host
	port:=contact.Port

	address := fmt.Sprintf("%s:%v", host.String(), port)
	portStr := fmt.Sprintf("%v", port)
	client, err := rpc.DialHTTPPath("tcp", address, rpc.DefaultRPCPath + portStr)
	if err != nil {
		return nil, nil, &CommandFailed{
		"Unable to find value " + fmt.Sprintf("%s:%v", host.String(), port)}
	}
	

	req := FindValueRequest{k.SelfContact, NewRandomID(), searchKey}
	var res FindValueResult
	err = client.Call("KademliaRPC.FindValue", &req, &res)
	if err != nil {
		return nil, nil, &CommandFailed{
		"Unable to find value" + fmt.Sprintf("%s:%v", host.String(), port)}
	} else {


		for _,node := range res.Nodes {

			err := k.Update(&node)
			if err !=nil {
				return nil, nil, err
			}
		}
		return res.Value, res.Nodes, res.Err
	}

	
}
func (k *Kademlia) NearestHelper(targetKey ID) (contacts []Contact, err error) {
	bucket_id :=k.NodeID.Xor(targetKey).PrefixLen()

	contacts = make([]Contact, 20, 20)
	ctr :=0
	for ctr < 20 && bucket_id < 160 {
		getReq := KBucketsMsg{Request: "get", Key: bucket_id}
		k.KBucketsReqChan <- getReq
		getRes := <- k.KBucketsResChan
		if getRes.Err != nil {
			log.Println("Error in update KBucketsReqChan: ", getRes.Err)
			return nil, getRes.Err
		}
		bucket := getRes.Value
	
		for i,bucketContact := range bucket.Contacts {
			contacts  = append(contacts, bucketContact)
			ctr = i
		}
		updateReq := KBucketsMsg{"update", bucket_id, bucket, nil}
		k.KBucketsReqChan <- updateReq
		updateRes := <- k.KBucketsResChan
		if updateRes.Err != nil {
			return nil, updateRes.Err
		}

		bucket_id += 1
	}
	return contacts, err 

}

func (k *Kademlia) LocalFindValue(searchKey ID) ([]byte, error) {
	getReq := VTableMsg{"get", searchKey, nil, nil}
	k.VTableReqChan <- getReq
	getRes := <- k.VTableResChan
	if getRes.Err != nil {
		return nil, getRes.Err
	}
	value := getRes.Value
	if value == nil {
		return nil, &CommandFailed{
		"Unable to find value LocalFindValue"}
	} else {
		return value, nil
	}
}

// For project 2!
func (k *Kademlia) DoIterativeFindNode(id ID) ([]Contact, error) {
	return nil, &CommandFailed{"Not implemented"}
}
func (k *Kademlia) DoIterativeStore(key ID, value []byte) ([]Contact, error) {
	return nil, &CommandFailed{"Not implemented"}
}
func (k *Kademlia) DoIterativeFindValue(key ID) (value []byte, err error) {
	return nil, &CommandFailed{"Not implemented"}
}

// For project 3!
func (k *Kademlia) Vanish(data []byte, numberKeys byte,
	threshold byte, timeoutSeconds int) (vdo VanashingDataObject) {
	return
}

func (k *Kademlia) Unvanish(searchKey ID) (data []byte) {
	return nil
}
