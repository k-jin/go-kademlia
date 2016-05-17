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
	"math"
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
	ShortlistDistanceChan chan int
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

// Request can be get or add
// Active indicates which array we are doing the request to 
type ShortlistMsg struct {
	Request 	string
	Active 		bool
	Contacts 	[]Contact
	Err 		error
}

// Struct for DoIterativeFindNode
type DoItFNMsg struct {
	DestContact 	Contact
	ResultContacts	[]Contact
	QuitChan 		chan bool
	Done 			bool
}

type ByDistance []Contact


// func (a ByDistance) Len() int           { return len(a) }
// func (a ByDistance) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
// func (a ByDistance) Less(i, j int) bool { 
// 	// var ai_dist uint64 = a[i].NodeID.Xor(target).PrefixLen()
// 	// var aj_dist uint64= a[j].NodeID.Xor(target).PrefixLen()
// 	// return ai_dist < aj_dist
// 	return a[i].Distance < a[j].Distance
// }

// May not be thread safe, consider adding "get/update" case
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
	k.ShortlistDistanceChan = make(chan int)

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
	
		for _,bucketContact := range bucket.Contacts {
			contacts  = append(contacts, bucketContact)
			ctr += 1
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
	goRequests := make(map[ID]DoItFNMsg)
	resultChan := make(chan DoItFNMsg)

	// query self for closest 20 nodes
	initContacts,err := k.DoFindNode(&k.SelfContact, id)
	if err != nil { return nil, err }

	// add closest 20 nodes to unchecked
	addReq := ShortlistMsg{"add", false, initContacts, nil}
	k.ShortlistReqChan <- addReq
	addRes := <- k.ShortlistResChan
	if addRes.Err != nil { return nil, addRes.Err }

	// get the 3 lowest unchecked nodes from the shortlist manager
	getInitReq := ShortlistMsg{"get", false, nil, nil}
	k.ShortlistReqChan <- getInitReq
	getInitRes := <- k.ShortlistResChan
	if getInitRes.Err != nil { return nil, getInitRes.Err }

	// initialize time for timer
	startTime := time.Now()
	// start threads for each node we get in response
	for _, contact := range getInitRes.Contacts {
		goRequests[contact.NodeID] = DoItFNMsg{contact, nil, make(chan bool), false}
		go k.DoFindNodeWrapper(goRequests[contact.NodeID].DestContact, id, resultChan, goRequests[contact.NodeID].QuitChan)

	}
	//TODO: deal with the case where we don't run 3 node requests
	for {
		select {
		case resultMsg := <- resultChan:
			responseContact := resultMsg.DestContact
			currMsg := goRequests[responseContact.NodeID]
			currMsg.Done = true
			goRequests[responseContact.NodeID] = currMsg


			// Find the shortest distance node within the results
			var minDistance = math.MaxInt32
			for _, contact := range resultMsg.ResultContacts {
				currDistance := contact.NodeID.Xor(id).PrefixLen()
				if currDistance < minDistance {
					minDistance = currDistance
				}
			}
			closestActiveReq := ShortlistMsg{"get", true, nil, nil}
			k.ShortlistReqChan <- closestActiveReq
			closestActiveDistance := <- k.ShortlistDistanceChan

			if minDistance < closestActiveDistance{
				// add result 20 nodes to unchecked
				addReq := ShortlistMsg{"add", false, resultMsg.ResultContacts, nil}
				k.ShortlistReqChan <- addReq
				addRes := <- k.ShortlistResChan
				if addRes.Err != nil { return nil, addRes.Err }
				// add contacted node to active nodes
				addReqRPC := ShortlistMsg{"add", true, []Contact{responseContact}, nil}
				k.ShortlistReqChan <- addReqRPC
				addResRPC := <- k.ShortlistResChan
				if addResRPC.Err != nil { return nil, addResRPC.Err }
				if len(addResRPC.Contacts) == 20 {
					return addResRPC.Contacts, nil
				}
			}
		default:
			cycleOver := false
			if time.Now().Sub(startTime) >= 300 * time.Millisecond {
				for _, msg := range goRequests {
					msg.QuitChan <- true
				}
				cycleOver = true
			} else {
				cycleOver = true
				for _, msg := range goRequests {
					cycleOver = cycleOver && msg.Done
				}
			}
			if cycleOver {
				goRequests := make(map[ID]DoItFNMsg)
				// get the 3 lowest unchecked nodes from the shortlist manager
				getInitReq := ShortlistMsg{"get", false, nil, nil}
				k.ShortlistReqChan <- getInitReq
				getInitRes := <- k.ShortlistResChan
				if getInitRes.Err != nil { return nil, getInitRes.Err}
				// initialize time for timer
				startTime = time.Now()

				// start threads for each node we get in response
				for _, contact := range getInitRes.Contacts {
					goRequests[contact.NodeID] = DoItFNMsg{contact, nil, make(chan bool), false}
					go k.DoFindNodeWrapper(goRequests[contact.NodeID].DestContact, id, resultChan, goRequests[contact.NodeID].QuitChan)
				}				
			}
		}
	}
	return nil, &CommandFailed{"Value not found"}
}
func (k *Kademlia) DoIterativeStore(key ID, value []byte) ([]Contact, error) {

	closestContacts, _ := k.DoIterativeFindNode(key)
	var storedContacts []Contact
	for _, contact := range closestContacts {
		err := k.DoStore(&contact, key, value)
		if err == nil {
			storedContacts = append(storedContacts, contact)
		}
	}
	return storedContacts, nil


	// for {
	// 	select {
	// 		case message := <-resultChan :
	// 			closestContacts := message.ResultContacts
	// 			var storedContacts []Contact
	// 			for _, contact := range closestContacts {
	// 				err := k.DoStore(&contact, key, value)
	// 				if err == nil {
	// 					storedContacts = append(storedContacts, contact)
	// 				}
	// 			}
	// 			return storedContacts, nil
	// 	}
	// }
	
	// return nil, &CommandFailed{"Not implemented"}
}
func (k *Kademlia) DoIterativeFindValue(key ID) (value []byte, err error) {
	return nil, &CommandFailed{"Not implemented"}
}

func (k *Kademlia) Merge(l []Contact, r []Contact, target ID) []Contact {
	ret := make([]Contact, 0, len(l)+len(r))
	for len(l) > 0 || len(r) > 0 {
		if len(l) == 0 {
			return append(ret, r...)
		}
		if len(r) == 0 {
			return append(ret, l...)
		}
		if l[0].NodeID.Xor(target).PrefixLen() <= r[0].NodeID.Xor(target).PrefixLen() {
			ret = append(ret, l[0])
			l = l[1:]
		} else {
			ret = append(ret, r[0])
			r = r[1:]
		}
	}
	return ret
}

func (k *Kademlia) MergeSort(s []Contact,target ID) []Contact {
	if len(s) <= 1 {
		return s
	}
	n := len(s) / 2
	l := k.MergeSort(s[:n],target)
	r := k.MergeSort(s[n:],target)
	
	return k.Merge(l, r, target)
}

// ShortlistMsg  
// "get" active==true - get the lowest active value 
// "get" active==false- get the 3 lowest unchecked value
// "add" active==true- put values into the active slice
// "add" active==false - put values into the unchecked slice

func (k *Kademlia) ShortlistManager(target ID) {
	// // These are arrays that are the bases for the slice versions
	// var active_arr [32]Contact
	// var unchecked_arr [128]Contact

	// These are the slices
	// active_slice := active_arr[0:0]
	// unchecked_slice := unchecked_arr[:40]

	active_slice := make([]Contact, 0)
	unchecked_slice := make([]Contact, 0)
	minDistance := math.MaxInt32
	for {
		req := <- k.ShortlistReqChan
		var res ShortlistMsg
		res = req
		res.Request = res.Request + " response"
		res.Err = nil
		fmt.Println("ShortlistManager Request")
		fmt.Println("Request Type: ", req.Request)
		fmt.Println("Active? ", req.Active)
		fmt.Println("Contacts: ", req.Contacts)
		// Add or get requests
		if req.Request == "add" {
			//TODO:  Update and maintain the minDistance
			// add to active or to unchecked slice
			if req.Active {
				fmt.Println("ADDING to active_slice: before")
				fmt.Println(active_slice)
				for _, contact:= range res.Contacts {
					active_slice = append(active_slice, contact)
				}
				fmt.Println("after")
				fmt.Println(active_slice)
				active_slice = k.MergeSort(active_slice, target)
				fmt.Println("sorted")
				fmt.Println(active_slice)
				res.Contacts = nil
				if len(active_slice) >= 20 {
					active_slice = active_slice[:20]
					res.Contacts = active_slice
				} 
			} else {
				fmt.Println("ADDING to unchecked_slice: before")
				fmt.Println(unchecked_slice)
				for _, contact:= range res.Contacts {
					unchecked_slice = append(unchecked_slice, contact)
				}
				fmt.Println("after")
				fmt.Println(unchecked_slice)
				unchecked_slice = k.MergeSort(unchecked_slice, target)
				fmt.Println("sorted")
				fmt.Println(unchecked_slice)
				if len(unchecked_slice) >= (20 - len(active_slice)) {
					fmt.Println("ACTIVE SLICE length", len(active_slice))
					unchecked_slice = unchecked_slice[:(20-len(active_slice) + 1)]
				}
				fmt.Println("truncated")
				fmt.Println(unchecked_slice)
				res.Contacts = unchecked_slice
			}
		} else if req.Request == "get" {
			if req.Active {
				fmt.Println("GETTING closest active contact")
				// if (len(active_slice) > 0) {
				// 	fmt.Println(active_slice[0])
				// 	res.Contacts = active_slice[0:1]
				k.ShortlistDistanceChan <- minDistance
				// ] else {
				// 	res.Err = &CommandFailed{"empty active_slice"}
				// }
			
			} else {
				fmt.Println("GETTING unchecked contacts")
				if len(unchecked_slice) >= 3 {
					fmt.Println(unchecked_slice[:3])
					res.Contacts = unchecked_slice[:3]
					unchecked_slice = unchecked_slice[3:]
				} else if len(unchecked_slice) == 2 {
					fmt.Println(unchecked_slice[:2])
					res.Contacts = unchecked_slice[:2]
					unchecked_slice = unchecked_slice[2:]
				} else if len(unchecked_slice) == 1 {
					fmt.Println(unchecked_slice[:1])
					res.Contacts = unchecked_slice[:1]
					unchecked_slice = unchecked_slice[1:]
				} else {
					fmt.Println("ERROR unchecked_slice empty")
					res.Err = &CommandFailed{"unchecked_slice empty"}
				}
			}
		} else {
			res.Err = &CommandFailed{"Improper request"}
		}
		k.ShortlistResChan <- res
	}
}

func (k *Kademlia) DoFindNodeWrapper(contact Contact, target ID, resChan chan DoItFNMsg, quitChan chan bool) {
	var base_arr = [20]Contact{}
	slice_results := base_arr[:]
	results,_ := k.DoFindNode(&contact, target)
	for _, item := range results {
		slice_results = append(slice_results, item)
		select {
			case <- quitChan:
				return
			default:
				continue
		}
	}
	resChan <- DoItFNMsg{contact, slice_results[:], nil, true}
	return

}
// For project 3!
func (k *Kademlia) Vanish(data []byte, numberKeys byte,
	threshold byte, timeoutSeconds int) (vdo VanashingDataObject) {
	return
}

func (k *Kademlia) Unvanish(searchKey ID) (data []byte) {
	return nil
}
