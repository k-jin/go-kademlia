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
	KBuckets map[int]KBucket
	ValueTable map[ID][]byte
}

func NewKademliaWithId(laddr string, nodeID ID) *Kademlia {
	k := new(Kademlia)
	k.NodeID = nodeID

	// TODO: Initialize other state here as you add functionality.
	k.KBuckets = make(map[int]KBucket)
	k.ValueTable = make(map[ID][]byte)
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
	return k
}

func NewKademlia(laddr string) *Kademlia {
	return NewKademliaWithId(laddr, NewRandomID())
}

// TODO maybe add error messages
func (k *Kademlia) Update(contact *Contact) error {

	if contact.NodeID == k.SelfContact.NodeID {
		return nil
	}
	bucket_id := contact.NodeID.Xor(k.SelfContact.NodeID).PrefixLen()
	// Is this a deep or shallow copy
	bucket := k.KBuckets[bucket_id]
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
	k.KBuckets[bucket_id] = bucket
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
	bucket := k.KBuckets[bucket_id]
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
	log.Printf("Pinging peer	\n")

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



	return &CommandFailed{"Not implemented"}
}

func (k *Kademlia) DoFindNode(contact *Contact, searchKey ID) ([]Contact, error) {
	// TODO: Implement
	return nil, &CommandFailed{"Not implemented"}
}

func (k *Kademlia) DoFindValue(contact *Contact,
	searchKey ID) (value []byte, contacts []Contact, err error) {
	// TODO: Implement
	return nil, nil, &CommandFailed{"Not implemented"}
}

func (k *Kademlia) LocalFindValue(searchKey ID) ([]byte, error) {
	// TODO: Implement
	return []byte(""), &CommandFailed{"Not implemented"}
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
