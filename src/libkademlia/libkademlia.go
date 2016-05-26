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
	"time"
	// "os"
	// "bufio"
	//"sss"
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
	ShortlistReqChan chan ShortlistMsg
	ShortlistResChan chan ShortlistMsg
	VDOReqChan chan VDOTableMsg
	VDOResChan chan VDOTableMsg
}

// Request can be get, add, update, or delete
type KBucketsMsg struct {
	Request 	string
	Key 		int
	Value 		KBucket
	Err 		error
}

// Request can be get, add, update, or delete
type VTableMsg struct {
	Request 	string
	Key 		ID
	Value 		[]byte
	Err 		error
}

// Request can be get, add, update, or delete
type VDOTableMsg struct {
	Request 	string
	Key 		ID
	Value 		VanashingDataObject
	Err 		error
}

// struct for communicating with ShortlistManager
type ShortlistMsg struct {
	// Request can be 'get' or 'add'
	Request 	string
	// true for active slice operations, false for unchecked slice operations
	Active 		bool
	// Contacts used either for requests or responses
	Contacts 	[]Contact
	Err 		error
}

// Struct for DoIterativeFindNode
// essentially tracks state of a FindNodeRPC
type DoItFNMsg struct {
	// Contact that RPC was called on
	DestContact 	Contact
	// Results from RPC
	ResultContacts	[]Contact
	// True if the RPC is part of the current cycle
	CurrentCycle	bool
	// True if this RPC has completed
	Done 			bool
	// True if none of the nodes returned by the RPC are closer than the ones in the shortlist
	TooFar			bool
	Err 			error
}

// Struct for DoIterativeFindValue
// essentially tracks state of a FindValueRPC
type DoItFVMsg struct {
	// Contact that RPC was called on
	DestContact 	Contact
	// Results from RPC
	ResultContacts	[]Contact
	// Value if found from RPC
	Value           []byte
	// True if the RPC is part of the current cycle
	CurrentCycle	bool
	// True if this RPC has completed
	Done 			bool
	// True if none of the nodes returned by the RPC are closer than the ones in the shortlist 
	TooFar			bool
	Err             error
}


func (k *Kademlia) KBucketsManager() {
	KBuckets := make(map[int]KBucket)
	for {
		req := <- k.KBucketsReqChan
		var res KBucketsMsg
		res = req
		res.Err = nil

		switch req.Request {
		case "get":
			res.Value = KBuckets[req.Key]
		case "update":
			KBuckets[req.Key] = req.Value
		case "add":
			KBuckets[req.Key] = req.Value
		case "delete":
			delete(KBuckets, res.Key)
		default:
			res.Err = &CommandFailed{"Invalid operation"}
		}
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
		switch req.Request {
		case "get":
			res.Value = ValueTable[req.Key]
		case "update":
			ValueTable[req.Key] = req.Value
		case "add":
			ValueTable[req.Key] = req.Value
		case "delete":
			delete(ValueTable, res.Key)
		default:
			res.Err = &CommandFailed{"Invalid operation"}
		}
		k.VTableResChan <- res
	}
}

func (k *Kademlia) VDOTableManager() {
	VDOTable := make(map[ID]VanashingDataObject)
	for {
		req := <- k.VDOReqChan
		var res VDOTableMsg
		res = req
		res.Err = nil
		switch req.Request {
		case "get":
			res.Value = VDOTable[req.Key]
		case "update":
			VDOTable[req.Key] = req.Value
		case "add":
			VDOTable[req.Key] = req.Value
		case "delete":
			delete(VDOTable, res.Key)
		default:
			res.Err = &CommandFailed{"Invalid operation"}
		}
		k.VDOResChan <- res
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
	k.ShortlistReqChan = make(chan ShortlistMsg)
	k.ShortlistResChan = make(chan ShortlistMsg)
	k.VDOReqChan = make(chan VDOTableMsg)
	k.VDOResChan = make(chan VDOTableMsg)

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
	go k.VDOTableManager()
	return k
}

func NewKademlia(laddr string) *Kademlia {
	return NewKademliaWithId(laddr, NewRandomID())
}

func (k *Kademlia) Update(contact *Contact) error {

	if contact.NodeID == k.SelfContact.NodeID {
		return nil
	}
	bucket_id := 159 - contact.NodeID.Xor(k.SelfContact.NodeID).PrefixLen()
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

	bucket_id := 159 - nodeId.Xor(k.SelfContact.NodeID).PrefixLen()
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
		// fmt.Println("dofindnode %v", port)
		// fmt.Println(result.Nodes)
		return result.Nodes, nil
	}

}

func (k *Kademlia) DoFindValue(contact *Contact,
	searchKey ID) (value []byte, contacts []Contact, err error) {
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
	bucket_id :=159 - k.NodeID.Xor(targetKey).PrefixLen()

	contacts = make([]Contact, 0)
	ctr := 0
	bucketsChecked := 0
	for ctr < 20 && bucketsChecked < 160 {
		getReq := KBucketsMsg{Request: "get", Key: bucket_id}
		k.KBucketsReqChan <- getReq
		getRes := <- k.KBucketsResChan
		if getRes.Err != nil {
			log.Println("Error in update KBucketsReqChan: ", getRes.Err)
			return nil, getRes.Err
		}
		bucket := getRes.Value
	
		for _,bucketContact := range bucket.Contacts {
			contacts = append(contacts, bucketContact)
			ctr += 1
		}
		updateReq := KBucketsMsg{"update", bucket_id, bucket, nil}
		k.KBucketsReqChan <- updateReq
		updateRes := <- k.KBucketsResChan
		if updateRes.Err != nil {
			return nil, updateRes.Err
		}

		bucket_id += 1
		bucket_id %= 160
		bucketsChecked += 1
	}
	// fmt.Println("nearest helper")
	// fmt.Println(contacts)
	// fmt.Println(len(contacts))
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

// Min function for int
func Min(x, y int) int {
	if x > y {
		return y
	} else {
		return x
	}
}

// Checks to see if target exists in contacts
func ContactExists(target Contact, contacts []Contact) bool {
	for _, contact := range contacts {
		if contact.NodeID == target.NodeID {
			return true
		}
	}
	return false
}

/*
	Shortlist 

		A go thread with a shortlist manager
			fields
				active []contacts 
				unchecked []contacts 
			- Main stopping condition: 20 active contacts (shortlist) found or all unchecked contacts have been checked 
			and no closer nodes are found to closest node (return what you have in shortlist/active contacts)
			1. Place 3 self closest contacts in unchecked
			2. while (!unchecked.isEmpty) send 3 RPC calls to first 3 contacts in unchecked
				- if RPC doesn't respond in 300ms, remove contact from unchecked (inactive)
				- else (rpc responds)
					- if all 20 contacts returned by RPC are further from closestNode, continue (don't add new contacts to unchecked)
					- else add all 20 contacts returned to unchecked, remove contact from unchecked, add to active, sort unchecked, 
					update closestNode, only keep the first 20-len(active) in unchecked, remove rest

*/


func (k *Kademlia) DoIterativeFindNode(id ID) ([]Contact, error) {
	// setup shortlist manager for accessing active and inactive shortlist items
	go k.ShortlistManager(id)

	// Initialize DoItFNMsg structs for each go thread
	// goRequests is a map of NodeIDs to DoItFNMsgs
	// DoItFNMsgs are structs that contain information on the state of each RPC call
	// resultChan is for passing the results of each RPC back to this function
	goRequests := make(map[ID]DoItFNMsg)
	resultChan := make(chan DoItFNMsg)

	// query self for closest k nodes
	initContacts,err := k.DoFindNode(&k.SelfContact, id)
	if err != nil { return nil, err }

	// add closest k nodes to unchecked
	addReq := ShortlistMsg{"add", false, initContacts, nil}
	k.ShortlistReqChan <- addReq
	addRes := <- k.ShortlistResChan
	if addRes.Err != nil { return nil, addRes.Err }

	// get the alpha closest unchecked nodes from the shortlist manager
	getInitReq := ShortlistMsg{"get", false, nil, nil}
	k.ShortlistReqChan <- getInitReq
	getInitRes := <- k.ShortlistResChan
	if getInitRes.Err != nil { return nil, getInitRes.Err }

	// initialize timer for cycles
	startTime := time.Now()

	// start threads for each alpha unchecked node
	for _, contact := range getInitRes.Contacts {
		goRequests[contact.NodeID] = DoItFNMsg{contact, nil, true, false, false, nil}
		go k.DoFindNodeWrapper(goRequests[contact.NodeID].DestContact, id, resultChan)
	}

	for {
		select {
		case resultMsg := <- resultChan:
			if resultMsg.Err != nil {
				return nil, resultMsg.Err
			}			
			// fmt.Println("result msg contacts")
			// fmt.Println(resultMsg.ResultContacts)
			responseContact := resultMsg.DestContact
			currMsg := goRequests[responseContact.NodeID]
			// Update struct DoItFNMsg to indicate this RPC has finished
			currMsg.Done = true

			minDistance := 200
			closestShortlistDistance := 200

			// Find the closest node within the results
			for _, contact := range resultMsg.ResultContacts {
				// fmt.Print("distance from ", contact.NodeID)
				// fmt.Print(" to ", id, " is ")
				currDistance := 159 - contact.NodeID.Xor(id).PrefixLen()
				// fmt.Println(currDistance)
				if currDistance < minDistance {
					minDistance = currDistance
				}
			}

			// Find the closest active node in shortlist
			closestActiveReq := ShortlistMsg{"get", true, nil, nil}
			k.ShortlistReqChan <- closestActiveReq
			closestActiveRes := <- k.ShortlistResChan
			if closestActiveRes.Err != nil { return nil, closestActiveRes.Err }
			if len(closestActiveRes.Contacts) > 0 {
				closestShortlistDistance = 159 - closestActiveRes.Contacts[0].NodeID.Xor(id).PrefixLen()
			}

			// Find the closest unchecked node in shortlist
			// If closer than the closest active node, update closestShortlistDistance
			closestUncheckedReq := ShortlistMsg{"get", false, nil, nil}
			k.ShortlistReqChan <- closestUncheckedReq
			closestUncheckedRes := <- k.ShortlistResChan
			if closestUncheckedRes.Err != nil { return nil, closestUncheckedRes.Err }
			if len(closestUncheckedRes.Contacts) > 0 {
				currDistance := 159 - closestUncheckedRes.Contacts[0].NodeID.Xor(id).PrefixLen()
				if currDistance < closestShortlistDistance {
					closestShortlistDistance = currDistance
				}
			}

			// if there are new contacts that are closer than the closest in the shortlist
			if minDistance <= closestShortlistDistance {
				// add result nodes to unchecked shortlist
				addUncheckedReq := ShortlistMsg{"add", false, resultMsg.ResultContacts, nil}
				k.ShortlistReqChan <- addUncheckedReq
				addUncheckedRes := <- k.ShortlistResChan
				if addUncheckedRes.Err != nil { return nil, addUncheckedRes.Err }
				
				// add contacted node to active shortlist
				addActiveReq := ShortlistMsg{"add", true, []Contact{responseContact}, nil}
				k.ShortlistReqChan <- addActiveReq
				addActiveRes := <- k.ShortlistResChan
				if addActiveRes.Err != nil { return nil, addActiveRes.Err }

				// if active shortlist has reached 20, return it
				if len(addActiveRes.Contacts) == 20 {
					// fmt.Println("full active shortlist")
					// fmt.Println(addActiveRes.Contacts)
					return addActiveRes.Contacts, nil
				}
			} else {
				// if there are no contacts that are closer than the closest in the shortlist
				currMsg.TooFar = true
			}
			// update the map version of the DoItFNMsg
			goRequests[responseContact.NodeID] = currMsg
		default:
			// cycleOver indicates the current cycle is over
			// 			 set to true when either the there is a timeout 
			// 			 or all the current cycle's RPCs return
			cycleOver := false

			// noCloser is true if all of the current cycle's RPCs indicated that there are 
			//			no more nodes that are closer than the ones in the shortlist
			noCloser := false

			// check for timeout
			if time.Now().Sub(startTime) >= 300 * time.Millisecond {
				// fmt.Println("timeout")
				cycleOver = true
			} else {
				// Check to see if all the RPCs have returned and if so, if 
				// they all returned values that were not closer than the closest value 
				// in the shortlist
				cycleOver = true
				noCloser = true
				for _, msg := range goRequests {
					if msg.CurrentCycle {
						cycleOver = cycleOver && msg.Done
						noCloser = noCloser && msg.TooFar
					}
				}
			}

			// if the cycle is over, check to see if all RPCs of the last cycle had returned
			// nodes that were farther away than the ones in the shortlist. 
			// if so, we return the current active shortlist
			// else, call alpha more RPCs on the next alpha closest unchecked
			// nodes on the shortlist
			if cycleOver {
				// The below commented code waits for user input before continuing to the next cycle
				// fmt.Println("NoCloser: ", noCloser)
				// fmt.Print("Press 'Enter' to continue...")
				// bufio.NewReader(os.Stdin).ReadBytes('\n')
				// fmt.Println("=================================================")
				if noCloser {
					getReq := ShortlistMsg{"get", true, nil, nil}
					k.ShortlistReqChan <- getReq
					getRes := <- k.ShortlistResChan
					if getRes.Err != nil { return nil, getRes.Err }
					// fmt.Println("no more closer")
					// fmt.Println(getRes.Contacts)
					return getRes.Contacts, nil
				} else {
					// fmt.Println("new cycle")
					for _, msg := range goRequests {
						msg.CurrentCycle = false
					}
					// get the alpha closest unchecked nodes from the shortlist manager
					getInitReq := ShortlistMsg{"get", false, nil, nil}
					k.ShortlistReqChan <- getInitReq
					getInitRes := <- k.ShortlistResChan
					if getInitRes.Err != nil { return nil, getInitRes.Err}

					// if the unchecked shortlist is empty, return the active shortlist
					if len(getInitRes.Contacts) == 0 {
						getReq := ShortlistMsg{"get", true, nil, nil}
						k.ShortlistReqChan <- getReq
						getRes := <- k.ShortlistResChan
						if getRes.Err != nil { return nil, getRes.Err }
						// fmt.Println("checked all nodes")
						// fmt.Println(getRes.Contacts)
						return getRes.Contacts, nil
					}

					// initialize time for timer
					startTime = time.Now()

					// start threads alpha nodes 
					for _, contact := range getInitRes.Contacts {
						goRequests[contact.NodeID] = DoItFNMsg{contact, nil, true, false, false, nil}
						go k.DoFindNodeWrapper(goRequests[contact.NodeID].DestContact, id, resultChan)
					}				
				}
				
			}
		}
	}
	return nil, &CommandFailed{"Nodes not found"}
}


func (k *Kademlia) DoIterativeStore(key ID, value []byte) ([]Contact, error) {
	// find the k closest contacts to key, to store value in
	closestContacts, err := k.DoIterativeFindNode(key)
	if err != nil { return nil, err }
	var storedContacts []Contact

	// store the value in each of the closest contacts
	for _, contact := range closestContacts {
		err = k.DoStore(&contact, key, value)
		if err != nil { return nil, err }
		storedContacts = append(storedContacts, contact)
	}
	// fmt.Println("DoItStore", storedContacts)
	return storedContacts, nil
}

func (k *Kademlia) DoIterativeFindValue(key ID) (value []byte, err error) {
	// setup shortlist manager for accessing active and inactive shortlist items
	go k.ShortlistManager(key)

	// Initialize DoItFVMsg structs for each go thread
	// goRequests is a map of NodeIDs to DoItFNMsgs
	// DoItFNMsgs are structs that contain information on the state of each RPC call
	// resultChan is for passing the results of each RPC back to this function
	goRequests := make(map[ID]DoItFVMsg)
	resultChan := make(chan DoItFVMsg)

	// query self for closest k nodes
	value, initContacts, err := k.DoFindValue(&k.SelfContact, key)
	if err != nil { return nil, err }
	if value != nil {return value, nil}

	// add closest k nodes to unchecked
	addReq := ShortlistMsg{"add", false, initContacts, nil}
	k.ShortlistReqChan <- addReq
	addRes := <- k.ShortlistResChan
	if addRes.Err != nil { return nil, addRes.Err }

	// get the alpha closest unchecked nodes from the shortlist manager
	getInitReq := ShortlistMsg{"get", false, nil, nil}
	k.ShortlistReqChan <- getInitReq
	getInitRes := <- k.ShortlistResChan
	if getInitRes.Err != nil { return nil, getInitRes.Err }

	// initialize time for cycles
	startTime := time.Now()

	// start threads for each each alpha unchecked node
	for _, contact := range getInitRes.Contacts {
		goRequests[contact.NodeID] = DoItFVMsg{contact, nil, value, true, false, false, nil}
		go k.DoFindValueWrapper(goRequests[contact.NodeID].DestContact, key, resultChan)
	}

	for {
		select {
		case resultMsg := <- resultChan:
			if resultMsg.Err != nil {
				return nil, resultMsg.Err
			} 
			// if DoFindValue returns the value, return it
			if resultMsg.Value != nil { 
				shortRequest := ShortlistMsg{"get", true, nil, nil}
				k.ShortlistReqChan <- shortRequest
				shortResponse := <- k.ShortlistResChan
				if shortResponse.Err != nil { return nil, shortResponse.Err }
				// Store value in closest Node
				// Check to make sure that the closest node isn't the node that returned the value
				if resultMsg.DestContact.NodeID == key {
					if len(shortResponse.Contacts) > 1 {
						k.DoStore(&shortResponse.Contacts[1], key, resultMsg.Value) 
					}
					return resultMsg.Value, nil
				} else {
					if len(shortResponse.Contacts) != 0 {
						k.DoStore(&shortResponse.Contacts[0], key, resultMsg.Value) 
					}
					return resultMsg.Value, nil
				}
			} 

			// fmt.Println("result msg contacts")
			// fmt.Println(resultMsg.ResultContacts)
			responseContact := resultMsg.DestContact
			currMsg := goRequests[responseContact.NodeID]
			// Update struct DoItFVMsg to indicate this RPC has finished
			currMsg.Done = true

			minDistance := 200
			closestShortlistDistance := 200
			// Find the closest node within the results
			for _, contact := range resultMsg.ResultContacts {
				currDistance := 159 - contact.NodeID.Xor(key).PrefixLen()
				if currDistance < minDistance {
					minDistance = currDistance
				}
			}

			// Find the closest active node in shortlist
			closestActiveReq := ShortlistMsg{"get", true, nil, nil}
			k.ShortlistReqChan <- closestActiveReq
			closestActiveRes := <- k.ShortlistResChan
			if closestActiveRes.Err != nil { return nil, closestActiveRes.Err }
			if len(closestActiveRes.Contacts) > 0 {
				closestShortlistDistance = 159 - closestActiveRes.Contacts[0].NodeID.Xor(key).PrefixLen()

			}

			// Find the closest unchecked node in shortlist
			// Iff closer than the closest active node, update closestShortlistDistance
			closestUncheckedReq := ShortlistMsg{"get", false, nil, nil}
			k.ShortlistReqChan <- closestUncheckedReq
			closestUncheckedRes := <- k.ShortlistResChan
			if closestUncheckedRes.Err != nil { return nil, closestUncheckedRes.Err }
			if len(closestUncheckedRes.Contacts) > 0 {
				currDistance := 159 - closestUncheckedRes.Contacts[0].NodeID.Xor(key).PrefixLen()
				if currDistance < closestShortlistDistance {
					closestShortlistDistance = currDistance
				}
			}

			// if there are new contacts that are closer than the closest in the shortlist
			if minDistance <= closestShortlistDistance {
				// add result nodes to unchecked shortlist
				addUncheckedReq := ShortlistMsg{"add", false, resultMsg.ResultContacts, nil}
				k.ShortlistReqChan <- addUncheckedReq
				addUncheckedRes := <- k.ShortlistResChan
				if addUncheckedRes.Err != nil { return nil, addUncheckedRes.Err }
				
				// add contacted node to active shortlist
				addActiveReq := ShortlistMsg{"add", true, []Contact{responseContact}, nil}
				k.ShortlistReqChan <- addActiveReq
				addActiveRes := <- k.ShortlistResChan
				if addActiveRes.Err != nil { return nil, addActiveRes.Err }

				// if active shortlist has reached 20, return it
				if len(addActiveRes.Contacts) == 20 {
					fmt.Println("full active shortlist")
					fmt.Println(addActiveRes.Contacts)
					if len(addActiveRes.Contacts) > 0 {
						return nil, &CommandFailed{fmt.Sprintf("Closest Active Node is: %v", addActiveRes.Contacts[0])}				
					} else {
						return nil, &CommandFailed{fmt.Sprintf("No Active Nodes")}
					}
				}
			} else {
				// if there are no contacts that are closer than the closest in the shortlist
				currMsg.TooFar = true
			}
			// update the map version of the DoItFnMsg
			goRequests[responseContact.NodeID] = currMsg

		default:
			// cycleOver indicates the current cycle is over
			// 			 set to true when either the there is a timeout 
			// 			 or all the current cycle's RPCs return
			cycleOver := false

			// noCloser is true if all of the current cycle's RPCs indicated that there are 
			//			no more nodes that are closer than the ones in the shortlist
			noCloser := false

			// check for timeout
			if time.Now().Sub(startTime) >= 300 * time.Millisecond {
				// fmt.Println("timeout")
				cycleOver = true
			} else {
				// Check to see if all the RPCs have returned and if so, if 
				// they all returned values that were not closer than the closest value 
				// in the shortlist
				cycleOver = true
				noCloser = true
				for _, msg := range goRequests {
					if msg.CurrentCycle {
						cycleOver = cycleOver && msg.Done
						noCloser = noCloser && msg.TooFar
					}
				}
			}

			// if the cycle is over, check to see if all RPCs of the last cycle had returned
			// nodes that were farther away than the ones in the shortlist. 
			// if so, we return the current active shortlist
			// else, call alpha more RPCs on the next alpha closest unchecked
			// nodes on the shortlist
			if cycleOver {
				if noCloser {
					getReq := ShortlistMsg{"get", true, nil, nil}
					k.ShortlistReqChan <- getReq
					getRes := <- k.ShortlistResChan
					if getRes.Err != nil { return nil, getRes.Err }
					fmt.Println("no more closer")
					fmt.Println(getRes.Contacts)
					if len(getRes.Contacts) > 0 {
						return nil, &CommandFailed{fmt.Sprintf("Closest Active Node is: %v", getRes.Contacts[0])}				
					} else {
						return nil, &CommandFailed{fmt.Sprintf("No Active Nodes")}
					}			
				} else {
					fmt.Println("new cycle")
					for _, msg := range goRequests {
						msg.CurrentCycle = false
					}
					// get the alpha closest unchecked nodes from the shortlist manager
					getInitReq := ShortlistMsg{"get", false, nil, nil}
					k.ShortlistReqChan <- getInitReq
					getInitRes := <- k.ShortlistResChan
					if getInitRes.Err != nil { return nil, getInitRes.Err}

					// if unchecked shortlist is empty, return the active shortlist
					if len(getInitRes.Contacts) == 0 {
						getReq := ShortlistMsg{"get", true, nil, nil}
						k.ShortlistReqChan <- getReq
						getRes := <- k.ShortlistResChan
						if getRes.Err != nil { return nil, getRes.Err }
						fmt.Println("checked all nodes")
						fmt.Println(getRes.Contacts)
						if len(getRes.Contacts) > 0 {
							return nil, &CommandFailed{fmt.Sprintf("Closest Active Node is: %v", getRes.Contacts[0])}				
						} else {
							return nil, &CommandFailed{fmt.Sprintf("No Active Nodes")}
						}	
					}

					// initialize time for timer
					startTime = time.Now()

					// start threads for each node we get in response
					for _, contact := range getInitRes.Contacts {
						goRequests[contact.NodeID] = DoItFVMsg{contact, nil, nil, true, false, false, nil}
						go k.DoFindValueWrapper(goRequests[contact.NodeID].DestContact, key, resultChan)

					}
				}				
			}
		}
	}
	return nil, &CommandFailed{"Value not found"}
}

// helper function for Mergesort
func (k *Kademlia) Merge(l []Contact, r []Contact, target ID) []Contact {
	ret := make([]Contact, 0, len(l)+len(r))
	for len(l) > 0 || len(r) > 0 {
		if len(l) == 0 {
			return append(ret, r...)
		}
		if len(r) == 0 {
			return append(ret, l...)
		}
		if (159 - l[0].NodeID.Xor(target).PrefixLen()) <= (159 - r[0].NodeID.Xor(target).PrefixLen()) {
			ret = append(ret, l[0])
			l = l[1:]
		} else {
			ret = append(ret, r[0])
			r = r[1:]
		}
	}
	return ret
}

// Mergesort for contacts based on distance from target
func (k *Kademlia) MergeSort(s []Contact,target ID) []Contact {
	if len(s) <= 1 {
		return s
	}
	n := len(s) / 2
	l := k.MergeSort(s[:n],target)
	r := k.MergeSort(s[n:],target)
	
	return k.Merge(l, r, target)
}

// Shortlist manager
// ShortlistMsg  
// "get" active==true - get the active shortlist 
// "get" active==false - get the alpha closest unchecked contacts
// "add" active==true - put values into the active slice
// "add" active==false - put values into the unchecked slice
func (k *Kademlia) ShortlistManager(target ID) {
	// slice of active nodes
	active_slice := make([]Contact, 0)

	// slice of unchecked nodes
	unchecked_slice := make([]Contact, 0)

	// slice of every seen nodes to avoid duplicates
	duplicate_slice := make([]Contact, 0)

	for {
		select {
			case req := <- k.ShortlistReqChan:
				var res ShortlistMsg
				res = req
				res.Request = res.Request + " response"
				res.Err = nil
				// fmt.Println("ShortlistManager Request")
				// fmt.Println("Request Type: ", req.Request)
				// fmt.Println("Active? ", req.Active)
				// fmt.Println("Contacts: ", req.Contacts)

				// Add or get requests
				if req.Request == "add" {
					// add to active or to unchecked slice
					if req.Active {
						// fmt.Println("ADDING to active_slice: before")
						// fmt.Println(active_slice)

						// if contact is not in active_slice, add it 
						for _, contact:= range res.Contacts {
							if !ContactExists(contact, active_slice){
								active_slice = append(active_slice, contact)
							}
						}

						// fmt.Println("after")
						// fmt.Println(active_slice)

						// sort slice
						active_slice = k.MergeSort(active_slice, target)

						// fmt.Println("sorted")
						// fmt.Println(active_slice)

						// if active_slice is full, trim
						if len(active_slice) >= 20 {
							active_slice = active_slice[:20]
						} 
						res.Contacts = active_slice	
					} else {
						// fmt.Println("ADDING to unchecked_slice: before")
						// fmt.Println(unchecked_slice)

						// add contacts to unchecked and duplicate slice if they have
						// not been seen yet
						for _, contact:= range res.Contacts {
							if contact.Host != nil {
								if !ContactExists(contact, duplicate_slice){
									duplicate_slice = append(duplicate_slice, contact)
									unchecked_slice = append(unchecked_slice, contact)
								}
							}
						}

						// fmt.Println("after")
						// fmt.Println(unchecked_slice)
						
						// sort unchecked_slice
						unchecked_slice = k.MergeSort(unchecked_slice, target)
						
						// fmt.Println("sorted")
						// fmt.Println(unchecked_slice)
						
						// trim unchecked_slice 
						if len(unchecked_slice) >= (20 - len(active_slice)) {
							// fmt.Println("ACTIVE SLICE length", len(active_slice))
							unchecked_slice = unchecked_slice[:(20-len(active_slice) + 1)]
						}
						// fmt.Println("truncated")
						// fmt.Println(unchecked_slice)
						res.Contacts = unchecked_slice
					}
				} else if req.Request == "get" {
					if req.Active {
						// fmt.Println("GETTING active slice")
						// fmt.Println(active_slice)
						res.Contacts = active_slice
					} else {
						// fmt.Println("GETTING unchecked contacts")
						// fmt.Println(unchecked_slice)
						sliceBound := Min(3, len(unchecked_slice))
						res.Contacts = unchecked_slice[:sliceBound]
						unchecked_slice = unchecked_slice[sliceBound:]
						// fmt.Println(unchecked_slice)
					}
				} else {
					res.Err = &CommandFailed{"Improper request"}
				}
				k.ShortlistResChan <- res
		}		
	}
}

// Wrapper for DoFindNode so that we can pass return values via channels
func (k *Kademlia) DoFindNodeWrapper(contact Contact, target ID, resChan chan DoItFNMsg) {
	slice_results := make([]Contact, 0)
	results,error := k.DoFindNode(&contact, target)
	for _, item := range results {
		slice_results = append(slice_results, item)
	}
	resChan <- DoItFNMsg{contact, slice_results[:], true, true, false, error}
	return

}

// wrapper for DoFindValue so that we can pass return values via channels
func (k *Kademlia) DoFindValueWrapper(contact Contact, target ID, resChan chan DoItFVMsg) {
	slice_results := make([]Contact, 0)
	value, closestContacts, error := k.DoFindValue(&contact, target)
	if value != nil {
		resChan <- DoItFVMsg{contact, nil, value, true, true, false, error}
	} else {
		for _, item := range closestContacts {
			slice_results = append(slice_results, item)
		}
		resChan <- DoItFVMsg{contact, slice_results[:], nil, true, true, false, error}
	}
	return
}

// For project 3!
func (k *Kademlia) Vanish(VDOID ID, data []byte, numberKeys byte,
	threshold byte, timeoutSeconds int) (vdo VanashingDataObject) {
	
	vdo = k.VanishData(data, numberKeys, threshold, timeoutSeconds)

	vdoReq := VDOTableMsg{"add", VDOID, vdo, nil}
	k.VDOReqChan <- vdoReq
	vdoRes := <- k.VDOResChan
	 if vdoRes.Err != nil {
	 	fmt.Println(vdoRes.Err)
	 } 
	return
}

// Implement UnvashishData. This is basically the same as the previous function, but in reverse. Use vdo.AccessKey and CalculateSharedKeyLocations to search for at least vdo.Threshold keys in the DHT. Use sss.Combine to recreate the key, K, and use decrypt to unencrypt vdo.Ciphertext.
func (k *Kademlia) Unvanish(searchKey ID, VDO ID) (data []byte) {


	return nil
}
