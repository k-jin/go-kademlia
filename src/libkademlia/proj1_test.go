package libkademlia

import (
	"bytes"
	"net"
	"strconv"
	"testing"
	//"time"
	"fmt"
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
	// tree structure;
	// A->B->tree
	/*
	         C
	      /
	  A-B -- D
	      \
	         E
	*/

		         
	instance1 := NewKademlia("localhost:8000")
	instance2 := NewKademlia("localhost:8001")
	instance3 := NewKademlia("localhost:8002")
	instance4 := NewKademlia("localhost:8003")
	instance5 := NewKademlia("localhost:8004")
	instance6 := NewKademlia("localhost:8005")
	instance7 := NewKademlia("localhost:8006")
	instance8 := NewKademlia("localhost:8007")
	instance9 := NewKademlia("localhost:8008")
	instance10 := NewKademlia("localhost:8009")
	instance11 := NewKademlia("localhost:8010")
	instance12 := NewKademlia("localhost:8011")
	instance13 := NewKademlia("localhost:8012")
	instance14 := NewKademlia("localhost:8013")
	instance15 := NewKademlia("localhost:8014")
	instance16 := NewKademlia("localhost:8015")
	instance17 := NewKademlia("localhost:8016")
	instance18 := NewKademlia("localhost:8017")
	instance19 := NewKademlia("localhost:8018")
	instance20 := NewKademlia("localhost:8019")
	instance21 := NewKademlia("localhost:8020")
	instance22 := NewKademlia("localhost:8021")

	host2, port2, _ := StringToIpPort("localhost:8001")
	host3, port3, _ := StringToIpPort("localhost:8002")
	host4, port4, _ := StringToIpPort("localhost:8003")
	host5, port5, _ := StringToIpPort("localhost:8004")
	host6, port6, _ := StringToIpPort("localhost:8005")
	host7, port7, _ := StringToIpPort("localhost:8006")
	host8, port8, _ := StringToIpPort("localhost:8007")
	host9, port9, _ := StringToIpPort("localhost:8008")
	host10, port10, _ := StringToIpPort("localhost:8009")
	host11, port11, _ := StringToIpPort("localhost:8010")
	host12, port12, _ := StringToIpPort("localhost:8011")
	host13, port13, _ := StringToIpPort("localhost:8012")
	host14, port14, _ := StringToIpPort("localhost:8013")
	host15, port15, _ := StringToIpPort("localhost:8014")
	host16, port16, _ := StringToIpPort("localhost:8015")
	host17, port17, _ := StringToIpPort("localhost:8016")
	host18, port18, _ := StringToIpPort("localhost:8017")
	host19, port19, _ := StringToIpPort("localhost:8018")
	host20, port20, _ := StringToIpPort("localhost:8019")
	host21, port21, _ := StringToIpPort("localhost:8020")
	host22, port22, _ := StringToIpPort("localhost:8021")


	instance1.DoPing(host2, port2)
	instance2.DoPing(host3, port3)
	instance3.DoPing(host4, port4)
	instance4.DoPing(host5, port5)
	instance5.DoPing(host6, port6)
	instance6.DoPing(host7, port7)
	instance7.DoPing(host8, port8)
	instance8.DoPing(host9, port9)
	instance9.DoPing(host10, port10)
	instance10.DoPing(host11, port11)
	instance11.DoPing(host12, port12)
	instance12.DoPing(host13, port13)
	instance13.DoPing(host14, port14)
	instance14.DoPing(host15, port15)
	instance15.DoPing(host16, port16)
	instance16.DoPing(host17, port17)
	instance17.DoPing(host18, port18)
	instance18.DoPing(host19, port19)
	instance19.DoPing(host20, port20)
	instance20.DoPing(host21, port21)
	instance21.DoPing(host22, port22)

	key := instance22.NodeID
	contacts, err := instance1.DoIterativeFindNode(key)
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

func TestItStore(t *testing.T) {
	// test Dostore() function and LocalFindValue() function
	instance1 := NewKademlia("localhost:8022")
	instance2 := NewKademlia("localhost:8023")
	instance3 := NewKademlia("localhost:8024")
	instance4 := NewKademlia("localhost:8025")
	instance5 := NewKademlia("localhost:8026")
	instance6 := NewKademlia("localhost:8027")
	instance7 := NewKademlia("localhost:8028")
	instance8 := NewKademlia("localhost:8029")
	instance9 := NewKademlia("localhost:8030")
	instance10 := NewKademlia("localhost:8031")
	instance11 := NewKademlia("localhost:8032")
	instance12 := NewKademlia("localhost:8033")
	instance13 := NewKademlia("localhost:8034")
	instance14 := NewKademlia("localhost:8035")
	instance15 := NewKademlia("localhost:8036")
	instance16 := NewKademlia("localhost:8037")
	instance17 := NewKademlia("localhost:8038")
	instance18 := NewKademlia("localhost:8039")
	instance19 := NewKademlia("localhost:8040")
	instance20 := NewKademlia("localhost:8041")
	instance21 := NewKademlia("localhost:8042")
	_ = NewKademlia("localhost:8043")

	host2, port2, _ := StringToIpPort("localhost:8023")
	host3, port3, _ := StringToIpPort("localhost:8024")
	host4, port4, _ := StringToIpPort("localhost:8025")
	host5, port5, _ := StringToIpPort("localhost:8026")
	host6, port6, _ := StringToIpPort("localhost:8027")
	host7, port7, _ := StringToIpPort("localhost:8028")
	host8, port8, _ := StringToIpPort("localhost:8029")
	host9, port9, _ := StringToIpPort("localhost:8030")
	host10, port10, _ := StringToIpPort("localhost:8031")
	host11, port11, _ := StringToIpPort("localhost:8032")
	host12, port12, _ := StringToIpPort("localhost:8033")
	host13, port13, _ := StringToIpPort("localhost:8034")
	host14, port14, _ := StringToIpPort("localhost:8035")
	host15, port15, _ := StringToIpPort("localhost:8036")
	host16, port16, _ := StringToIpPort("localhost:8037")
	host17, port17, _ := StringToIpPort("localhost:8038")
	host18, port18, _ := StringToIpPort("localhost:8039")
	host19, port19, _ := StringToIpPort("localhost:8040")
	host20, port20, _ := StringToIpPort("localhost:8041")
	host21, port21, _ := StringToIpPort("localhost:8042")
	host22, port22, _ := StringToIpPort("localhost:8043")


	instance1.DoPing(host2, port2)
	instance2.DoPing(host3, port3)
	instance3.DoPing(host4, port4)
	instance4.DoPing(host5, port5)
	instance5.DoPing(host6, port6)
	instance6.DoPing(host7, port7)
	instance7.DoPing(host8, port8)
	instance8.DoPing(host9, port9)
	instance9.DoPing(host10, port10)
	instance10.DoPing(host11, port11)
	instance11.DoPing(host12, port12)
	instance12.DoPing(host13, port13)
	instance13.DoPing(host14, port14)
	instance14.DoPing(host15, port15)
	instance15.DoPing(host16, port16)
	instance16.DoPing(host17, port17)
	instance17.DoPing(host18, port18)
	instance18.DoPing(host19, port19)
	instance19.DoPing(host20, port20)
	instance20.DoPing(host21, port21)
	instance21.DoPing(host22, port22)

	// contact2, err := instance1.FindContact(instance2.NodeID)
	// if err != nil {
	// 	t.Error("Instance 2's contact not found in Instance 1's contact list")
	// 	return
	// }
	key := NewRandomID()
	value := []byte("Hello World")
	contacts, err := instance1.DoIterativeStore(key, value)
	fmt.Println(contacts)
	if err != nil {
		t.Error("Can not store this value")
	}
	storedValue, err := instance10.LocalFindValue(key)
	if err != nil {
		t.Error("Stored value not found!")
	}
	if !bytes.Equal(storedValue, value) {
		t.Error("Stored value did not match found value")
	}
	return
}
