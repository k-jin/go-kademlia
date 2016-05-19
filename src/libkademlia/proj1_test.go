package libkademlia

import (
	"bytes"
	"net"
	"strconv"
	"testing"
	//"time"
	//"fmt"
)

func StringToIpPort(laddr string) (ip net.IP, port uint16, err error) {
	hostString, portString, err := net.SplitHostPort(laddr)
	if err != nil {
		return
	}
	ipStr, err := net.LookupHost(hostString)
	if err != nil {
		return
	}
	for i := 0; i < len(ipStr); i++ {
		ip = net.ParseIP(ipStr[i])
		if ip.To4() != nil {
			break
		}
	}
	portInt, err := strconv.Atoi(portString)
	port = uint16(portInt)
	return
}

func TestPing(t *testing.T) {
	instance1 := NewKademlia("localhost:7890")
	instance2 := NewKademlia("localhost:7891")
	host2, port2, _ := StringToIpPort("localhost:7891")
	contact2, err := instance2.FindContact(instance2.NodeID)
	if err != nil {
		t.Error("A node cannot find itself's contact info")
	}
	contact2, err = instance2.FindContact(instance1.NodeID)
	if err == nil {
		t.Error("Instance 2 should not be able to find instance " +
			"1 in its buckets before ping instance 1")
	}
	instance1.DoPing(host2, port2)
	contact2, err = instance1.FindContact(instance2.NodeID)
	if err != nil {
		t.Error("Instance 2's contact not found in Instance 1's contact list")
		return
	}
	wrong_ID := NewRandomID()
	_, err = instance2.FindContact(wrong_ID)
	if err == nil {
		t.Error("Instance 2 should not be able to find a node with the wrong ID")
	}

	contact1, err := instance2.FindContact(instance1.NodeID)
	if err != nil {
		t.Error("Instance 1's contact not found in Instance 2's contact list")
		return
	}
	if contact1.NodeID != instance1.NodeID {
		t.Error("Instance 1 ID incorrectly stored in Instance 2's contact list")
	}
	if contact2.NodeID != instance2.NodeID {
		t.Error("Instance 2 ID incorrectly stored in Instance 1's contact list")
	}
	return
}

func TestStore(t *testing.T) {
	// test Dostore() function and LocalFindValue() function
	instance1 := NewKademlia("localhost:7892")
	instance2 := NewKademlia("localhost:7893")
	host2, port2, _ := StringToIpPort("localhost:7893")
	instance1.DoPing(host2, port2)
	contact2, err := instance1.FindContact(instance2.NodeID)
	if err != nil {
		t.Error("Instance 2's contact not found in Instance 1's contact list")
		return
	}
	key := NewRandomID()
	value := []byte("Hello World")
	err = instance1.DoStore(contact2, key, value)
	if err != nil {
		t.Error("Can not store this value")
	}
	storedValue, err := instance2.LocalFindValue(key)
	if err != nil {
		t.Error("Stored value not found!")
	}
	if !bytes.Equal(storedValue, value) {
		t.Error("Stored value did not match found value")
	}
	return
}

func TestFindNode(t *testing.T) {
	// tree structure;
	// A->B->tree
	/*
	         C
	      /
	  A-B -- D
	      \
	         E
	*/
	instance1 := NewKademlia("localhost:7894")
	instance2 := NewKademlia("localhost:7895")
	host2, port2, _ := StringToIpPort("localhost:7895")
	instance1.DoPing(host2, port2)
	contact2, err := instance1.FindContact(instance2.NodeID)
	// _, err := instance1.FindContact(instance2.NodeID)
	if err != nil {
		t.Error("Instance 2's contact not found in Instance 1's contact list")
		return
	}
	tree_node := make([]*Kademlia, 10)
	for i := 0; i < 10; i++ {
		address := "localhost:" + strconv.Itoa(7896+i)
		tree_node[i] = NewKademlia(address)
		host_number, port_number, _ := StringToIpPort(address)
		instance2.DoPing(host_number, port_number)
	}
	key := NewRandomID()
	// contacts, err := instance1.DoIterativeFindNode(key)
	contacts, err := instance1.DoFindNode(contact2, key)
	if err != nil {
		t.Error("Error doing FindNode")
	}

	if contacts == nil || len(contacts) == 0 {
		t.Error("No contacts were found")
	}
	// TODO: Check that the correct contacts were stored
	//       (and no other contacts)	

	return
}

func TestFindValue(t *testing.T) {
	// tree structure;
	// A->B->tree
	/*
	         C
	      /
	  A-B -- D
	      \
	         E
	*/
	instance1 := NewKademlia("localhost:7926")
	instance2 := NewKademlia("localhost:7927")
	host2, port2, _ := StringToIpPort("localhost:7927")
	instance1.DoPing(host2, port2)
	contact2, err := instance1.FindContact(instance2.NodeID)
	if err != nil {
		t.Error("Instance 2's contact not found in Instance 1's contact list")
		return
	}

	tree_node := make([]*Kademlia, 10)
	for i := 0; i < 10; i++ {
		address := "localhost:" + strconv.Itoa(7928+i)
		tree_node[i] = NewKademlia(address)
		host_number, port_number, _ := StringToIpPort(address)
		instance2.DoPing(host_number, port_number)
	}

	key := NewRandomID()
	value := []byte("Hello world")
	err = instance2.DoStore(contact2, key, value)
	if err != nil {
		t.Error("Could not store value")
	}

	// Given the right keyID, it should return the value
	foundValue, contacts, err := instance1.DoFindValue(contact2, key)
	if !bytes.Equal(foundValue, value) {
		t.Error("Stored value did not match found value")
	}

	//Given the wrong keyID, it should return k nodes.
	wrongKey := NewRandomID()
	foundValue, contacts, err = instance1.DoFindValue(contact2, wrongKey)
	if contacts == nil || len(contacts) < 10 {
		t.Error("Searching for a wrong ID did not return contacts")
	}

	// TODO: Check that the correct contacts were stored
	//       (and no other contacts)
}
func TestItFindNode(t *testing.T) {
	// This function tests the DoIterativeFindNode function
	// 22 kademlia instances are created and stored in a slice. 21 hosts and ports are stored in slices too.
	// We call DoIterativeFindNode on one instance(first in the instance slice) and ask it to find the last instance in the slice. 
	// If there is an error or no contacts are returned we handle the cases accordingly. 


	kinstances_slice := make([]Kademlia,0) // 1 to 21
	host_slice:=make([]net.IP,0) // 2 to 22
	port_slice:=make([]uint16,0) // 2 to 22

	for i:=0;i<22;i++ {
		port_string := "localhost:"+strconv.Itoa(8000+i)
		kinstances_slice = append(kinstances_slice, *NewKademlia(port_string))
		if i > 0 {
			temp_host, temp_port, _ := StringToIpPort(port_string)
			port_slice = append(port_slice,temp_port )
			host_slice = append(host_slice,temp_host )
		}		

	}
	for i:=0;i<21;i++ {
		kinstances_slice[i].DoPing(host_slice[i],port_slice[i])
	}
	key := kinstances_slice[21].NodeID
	contacts, err := kinstances_slice[0].DoIterativeFindNode(key)
	if err != nil {
		t.Error("Error doing FindNode")
	}

	if contacts == nil || len(contacts) == 0 {
		t.Error("No contacts were found")
	}
	// TODO: Check that the correct contacts were stored
	//       (and no other contacts)	

	return
}

// func TestItStore(t *testing.T) {
// 	// test Dostore() function and LocalFindValue() function

// 	kinstances_slice := make([]Kademlia,0) 
// 	host_slice:=make([]net.IP,0) 
// 	port_slice:=make([]uint16,0) 
// 	id2instance := make(map[ID]Kademlia)





// 	for i:=23;i<45;i++ {
// 		port_string := "localhost:"+strconv.Itoa(8000+i)
// 		temp_instance := *NewKademlia(port_string)
// 		kinstances_slice = append(kinstances_slice, temp_instance)
// 		id2instance[temp_instance.NodeID] = temp_instance
// 		if i > 23 {
// 			temp_host, temp_port, _ := StringToIpPort(port_string)
// 			port_slice = append(port_slice,temp_port )
// 			host_slice = append(host_slice,temp_host )

// 		}
// 	}

// 	for i:=23;i<44;i++ {
// 		kinstances_slice[i].DoPing(host_slice[i],port_slice[i])
// 	}

	
// 	key := kinstances_slice[21].NodeID
// 	value := []byte("Hello World")
// 	contacts, err := kinstances_slice[0].DoIterativeStore(key, value)

// 	fmt.Println(contacts)
// 	if err != nil {
// 		t.Error("Can not store this value")
// 	}
// 	for _,contact := range contacts {
// 		temp2_instance := id2instance[contact.NodeID]
// 		storedValue, err := temp2_instance.LocalFindValue(key)
// 		if err != nil {
// 			t.Error("Stored value not found!")
// 		}
// 		if !bytes.Equal(storedValue, value) {
// 			t.Error("Stored value did not match found value")
// 		}
// 	}
// 	return
// }
// func TestItFindValue(t *testing.T) {
// 	// tree structure;
// 	// A->B->tree
// 	/*
// 	         C
// 	      /
// 	  A-B -- D
// 	      \
// 	         E
// 	*/

// 	kinstances_slice := make([]Kademlia,0) 
// 	host_slice:=make([]net.IP,0) 
// 	port_slice:=make([]uint16,0) 
// 	id2instance := make(map[ID]Kademlia)

// 	for i:=50;i<62;i++ {
// 		port_string := "localhost:"+strconv.Itoa(8000+i)
// 		temp_instance := *NewKademlia(port_string)
// 		kinstances_slice = append(kinstances_slice, temp_instance)
// 		id2instance[temp_instance.NodeID] = temp_instance
// 		if i > 50 {
// 			temp_host, temp_port, _ := StringToIpPort(port_string)
// 			port_slice = append(port_slice,temp_port )
// 			host_slice = append(host_slice,temp_host )

// 		}
// 	}

// 	for i:=50;i<61;i++ {
// 		kinstances_slice[i].DoPing(host_slice[i],port_slice[i])
// 	}


// 	// instance1 := NewKademlia("localhost:7926")
// 	// instance2 := NewKademlia("localhost:7927")
// 	// host2, port2, _ := StringToIpPort("localhost:7927")
// 	// instance1.DoPing(host2, port2)



// 	contact2, err := kinstances_slice[0].FindContact(kinstances_slice[20].NodeID)
// 	if err != nil {
// 		t.Error("Instance 2's contact not found in Instance 1's contact list")
// 		return
// 	}

// 	tree_node := make([]*Kademlia, 10)
// 	for i := 0; i < 10; i++ {
// 		address := "localhost:" + strconv.Itoa(8065+i)
// 		tree_node[i] = NewKademlia(address)
// 		host_number, port_number, _ := StringToIpPort(address)
// 		kinstances_slice[20].DoPing(host_number, port_number)
// 	}

// 	key := kinstances_slice[20].NodeID
// 	value := []byte("Hello world")
// 	err = kinstances_slice[20].DoStore(contact2, key, value)
// 	if err != nil {
// 		t.Error("Could not store value")
// 	}

// 	// Given the right keyID, it should return the value
// 	foundValue, contacts, err := kinstances_slice[0].DoFindValue(contact2, key)
// 	if !bytes.Equal(foundValue, value) {
// 		t.Error("Stored value did not match found value")
// 	}

// 	//Given the wrong keyID, it should return k nodes.
// 	wrongKey := NewRandomID()
// 	foundValue, contacts, err = kinstances_slice[0].DoFindValue(contact2, wrongKey)
// 	if contacts == nil || len(contacts) < 10 {
// 		t.Error("Searching for a wrong ID did not return contacts")
// 	}

// 	// TODO: Check that the correct contacts were stored
// 	//       (and no other contacts)
// }
