package libkademlia

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	mathrand "math/rand"
	"time"
	"sss"
	"fmt"
)

type VanashingDataObject struct {
	AccessKey  int64
	Ciphertext []byte
	NumberKeys byte
	Threshold  byte
}

func GenerateRandomCryptoKey() (ret []byte) {
	for i := 0; i < 32; i++ {
		ret = append(ret, uint8(mathrand.Intn(256)))
	}
	return
}

func GenerateRandomAccessKey() (accessKey int64) {
	r := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	accessKey = r.Int63()
	return
}

func CalculateSharedKeyLocations(accessKey int64, count int64) (ids []ID) {
	r := mathrand.New(mathrand.NewSource(accessKey))
	ids = make([]ID, count)
	for i := int64(0); i < count; i++ {
		for j := 0; j < IDBytes; j++ {
			ids[i][j] = uint8(r.Intn(256))
		}
	}
	return
}

func encrypt(key []byte, text []byte) (ciphertext []byte) {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	ciphertext = make([]byte, aes.BlockSize+len(text))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(err)
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], text)
	return
}

func decrypt(key []byte, ciphertext []byte) (text []byte) {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	if len(ciphertext) < aes.BlockSize {
		panic("ciphertext is not long enough")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	return ciphertext
}

func (k *Kademlia) VanishData(data []byte, numberKeys byte,
	threshold byte, timeoutSeconds int) (vdo VanashingDataObject) {

	testVanishStoredNodes := make([]Contact, 0)
	cryptoKeyK := GenerateRandomCryptoKey()
	cipherTextC := encrypt(cryptoKeyK, data)
	splitKey, err := sss.Split(numberKeys, threshold, cryptoKeyK)
	if err != nil {
		fmt.Println("sss.Split messed up", err) 
		return
	}
	accessKeyL := GenerateRandomAccessKey()
	storeIds := CalculateSharedKeyLocations(accessKeyL, int64(numberKeys))
	
	// allSlice := make([][]byte, 0)
	allSlice := [][]byte{}


	ctr := 0
	for k,v := range splitKey {
		keyVal := append([]byte{k}, v...)
		allSlice= append(allSlice, keyVal)
		// ctr = ctr + 1
		fmt.Println("Ctr is ", ctr)
		fmt.Println("k is ",k)
		fmt.Println("v is ", v)

	}

	for i,id := range storeIds {
		storedAt, err := k.DoIterativeStore(id, allSlice[i])
		if err != nil {
			fmt.Println("DoIterativeStore messed up ", id, allSlice[i])
			return
		}
		testVanishStoredNodes = append(testVanishStoredNodes, storedAt...)
	}
	fmt.Println(cryptoKeyK)
	fmt.Println(cipherTextC)
	fmt.Println(testVanishStoredNodes)
	vdo = VanashingDataObject{accessKeyL, cipherTextC, numberKeys, threshold}
	fmt.Println(vdo)

	return
}

// Implement UnvashishData. This is basically the same as the previous function, but in reverse. Use vdo.AccessKey and CalculateSharedKeyLocations to search for at least vdo.Threshold keys in the DHT. Use sss.Combine to recreate the key, K, and use decrypt to unencrypt vdo.Ciphertext.
func (k *Kademlia) UnvanishData(vdo VanashingDataObject) (data []byte) {
	storeIds := CalculateSharedKeyLocations(vdo.AccessKey, int64(vdo.Threshold))
	fmt.Println("storedIDs UNVANISH: ", storeIds)
	keyParts := make(map[byte][]byte)
	for _, id := range storeIds {
		keyPart, err := k.DoIterativeFindValue(id)
		fmt.Println("Keypart from DoItFind ",keyPart)
		fmt.Println("Keyparts Map ", keyParts)
		if err != nil || len(keyPart)==0 {
			fmt.Println("Error is ", err)
			fmt.Println("Key not found at node ", id)
		} else if len(keyParts) < int(vdo.Threshold) {
			fmt.Println(keyPart)
			// fmt.Println(keyPart[0])
			// fmt.Println(keyPart[1:])
			keyParts[keyPart[0]] = keyPart[1:]
		} else {
			break
		}
	}
	if len(keyParts) < int(vdo.Threshold) {
		fmt.Println("keyParts is not big enough to Combine")
		return nil
	}

	fmt.Println("vdo.Threshold is  ",int(vdo.Threshold))
	fmt.Println("Keyparts is ",keyParts)
	key := sss.Combine(keyParts)
	fmt.Println("Key sent to decrypt is ",key)
	data = decrypt(key, vdo.Ciphertext)
	return
}
