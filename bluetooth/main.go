// package main

// import (
// 	"context"
// 	"fmt"
// 	"log"

// 	"encoding/hex"

// 	"github.com/go-ble/ble"
// 	"github.com/go-ble/ble/examples/lib/dev"
// )

// func main() {
// 	device, err := dev.NewDevice("default")
// 	if err != nil {
// 		log.Fatalf("can't create new device : %s", err)
// 	}
// 	ble.SetDefaultDevice(device)

// 	// MAC-адрес Bluetooth-устройства, к которому необходимо подключиться.
// 	addr := ble.NewAddr("A6:01:01:03:12:1A")

// 	// Установка контекста с таймаутом для подключения
// 	ctx := ble.WithSigHandler(context.Background(), func() {
// 		fmt.Println("Caught signal, shutting down...")
// 	})

// 	// Подключение к устройству по MAC-адресу
// 	client, err := ble.Dial(ctx, addr)
// 	if err != nil {
// 		log.Fatalf("can't dial to %s: %s", addr, err)
// 	}
// 	defer client.CancelConnection()

// 	fmt.Printf("Connected to %s\n", addr)
// 	// Теперь вы можете взаимодействовать с клиентом, например, обнаруживать его услуги и характеристики
// 	// ...
// 	uuid := ble.MustParse("2a00")
// 	characteristic, err := findCharacteristic(client, uuid)
// 	if err != nil {
// 		log.Fatalf("can't find characteristic: %s", err)
// 	}

// 	// Пишем данные на характеристику
// 	dataToSend := getColor("ffff")
// 	fmt.Println(dataToSend)
// 	err = client.WriteCharacteristic(characteristic, getColor("ff0000"), false)
// 	if err != nil {
// 		log.Fatalf("can't write to characteristic: %s", err)
// 	}

// 	fmt.Println("Data written to characteristic.")
// }

// // findCharacteristic находит и возвращает характеристику по её UUID.
// func findCharacteristic(client ble.Client, uuid ble.UUID) (*ble.Characteristic, error) {
// 	profile, err := client.DiscoverProfile(true)
// 	if err != nil {
// 		return nil, fmt.Errorf("can't discover profile: %w", err)
// 	}

// 	for _, service := range profile.Services {
// 		for _, characteristic := range service.Characteristics {
// 			fmt.Println(characteristic)
// 			if characteristic.UUID.Equal(uuid) {
// 				return characteristic, nil
// 			}
// 		}
// 	}

// 	return nil, fmt.Errorf("characteristic with UUID %s not found", uuid)
// }

// func getColor(color string) []byte {
// 	// sampleColorString := "ff0000"
// 	base := "6996060101"
// 	concate := base + color
// 	result, _ := hexStringToBytes(concate)
// 	return result
// }

// func hexStringToBytes(s string) ([]byte, error) {
// 	if len(s)%2 != 0 {
// 		return nil, fmt.Errorf("hex string must have an even length")
// 	}

// 	b, err := hex.DecodeString(s)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to decode hex string: %w", err)
// 	}

// 	return b, nil
// }

package main

import (
	"fmt"

	"tinygo.org/x/bluetooth"
)

type data struct {
	off   []byte
	on    []byte
	color []byte
}

var sampleData = data{
	off: []byte{0x69, 0x96, 0x02, 0x01, 0x00},
	on:  []byte{0x69, 0x96, 0x02, 0x01, 0x01},
	//6996060101ffff to color red
	color: []byte{0x69, 0x96, 0x06, 0x01, 0x01, 0xff, 0xff},
}

var adapter = bluetooth.DefaultAdapter

func main() {
	adapter.Enable()
	var bluetoothChar bluetooth.DeviceCharacteristic
	addrString := "A6:01:01:03:12:1A"
	adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		fmt.Printf("Device found: %s\n", device.Address.String())
		if device.Address.String() == addrString {
			fmt.Println("Device found")
			adapter.StopScan()
			var err error
			device, err := adapter.Connect(device.Address, bluetooth.ConnectionParams{})
			if err != nil {
				fmt.Printf("Failed to connect to device: %s\n", err)
				return
			}
			fmt.Println("Connected to device")
			// find service with UUID 2a00
			// find characteristic with UUID 2a00
			services, err := device.DiscoverServices([]bluetooth.UUID{})
			if err != nil {
				fmt.Printf("Failed to discover services: %s\n", err)
				return
			}
			fmt.Println("Services discovered")
			for _, service := range services {
				fmt.Printf("Service: %s\n", service.UUID().String())
				service.DiscoverCharacteristics([]bluetooth.UUID{})
				characteristics, err := service.DiscoverCharacteristics([]bluetooth.UUID{})
				if err != nil {
					fmt.Printf("Failed to discover characteristics: %s\n", err)
					return
				}
				fmt.Println("Characteristics discovered")
				for _, characteristic := range characteristics {
					fmt.Printf("Characteristic: %s\n", characteristic.UUID().String())
					if characteristic.UUID().String() == "0000ee01-0000-1000-8000-00805f9b34fb" {
						fmt.Println("Characteristic found")
						bluetoothChar = characteristic
						characteristic.WriteWithoutResponse(sampleData.color)
						fmt.Println("Characteristic written")
					}

					// write to characteristic
					// adapter.WriteCharacteristic(characteristic, data, false)
				}
			}
		}
		bluetoothChar.WriteWithoutResponse(sampleData.off)
	})

	// find service with UUID 2a00
	// find characteristic with UUID 2a00

	// write to characteristic
	// adapter.WriteCharacteristic(characteristic, data, false)
}
