package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/gordonklaus/portaudio"
	"tinygo.org/x/bluetooth"

	"encoding/json"
	"net/http"
)

const (
	NumInputChannels = 1
	StateOff         = "off"
	StateOn          = "on"
)

type Config struct {
	SampleRate      float64
	Threshold       float64
	BufferSize      int
	CutoffFrequency float64
	Delay           int
	Color           string
	Background      string
}

var cfg = Config{
	SampleRate:      44100,
	Threshold:       800,
	BufferSize:      512,
	CutoffFrequency: 50,
	Delay:           3,
	Color:           "ff0000",
	Background:      "000000",
}

func init() {
	flag.Float64Var(&cfg.SampleRate, "sampleRate", cfg.SampleRate, "Sample rate of the audio")
	flag.Float64Var(&cfg.Threshold, "threshold", cfg.Threshold, "Volume threshold for triggering an action")
	flag.IntVar(&cfg.BufferSize, "bufferSize", cfg.BufferSize, "Size of the audio buffer")
	flag.Float64Var(&cfg.CutoffFrequency, "cutoffFrequency", cfg.CutoffFrequency, "Cutoff frequency for the low-pass filter")
	flag.IntVar(&cfg.Delay, "delay", cfg.Delay, "Delay between audio buffer reads")
	flag.StringVar(&cfg.Color, "color", cfg.Color, "Color for the LED")
	flag.StringVar(&cfg.Background, "background", cfg.Background, "Background color for the LED")
}

type PeakTracker struct {
	state          string
	isRising       bool
	peakStartLevel float64
	peakEndLevel   float64
	lastState      string
}

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

func GetConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func PatchConfigHandler(w http.ResponseWriter, r *http.Request) {
	// Temporary struct to hold the incoming patch request
	var patchConfig struct {
		SampleRate      *float64 `json:"sampleRate,omitempty"`
		Threshold       *float64 `json:"threshold,omitempty"`
		BufferSize      *int     `json:"bufferSize,omitempty"`
		CutoffFrequency *float64 `json:"cutoffFrequency,omitempty"`
		Delay           *int     `json:"delay,omitempty"`
		Color           *string  `json:"color,omitempty"`
	}

	// Decode the incoming JSON to the temporary struct
	err := json.NewDecoder(r.Body).Decode(&patchConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Apply changes to the global configuration if they are present in the patch request
	if patchConfig.SampleRate != nil {
		cfg.SampleRate = *patchConfig.SampleRate
		// Apply change to the audio stream if necessary
	}
	if patchConfig.Threshold != nil {
		cfg.Threshold = *patchConfig.Threshold
		// Apply change to the audio stream if necessary
	}

	if patchConfig.BufferSize != nil {
		cfg.BufferSize = *patchConfig.BufferSize
		// Apply change to the audio stream if necessary
	}
	if patchConfig.CutoffFrequency != nil {
		cfg.CutoffFrequency = *patchConfig.CutoffFrequency
	}

	if patchConfig.Delay != nil {
		cfg.Delay = *patchConfig.Delay
		// Apply change to the audio stream if necessary
	}
	if patchConfig.Color != nil {
		cfg.Color = *patchConfig.Color
		// Apply change to the audio stream if necessary
	}

	// After updating the configuration, apply these changes to the active stream
	// This is where you need to add your specific logic to update the audio stream
	// For example, if the buffer size or sample rate changes, you might need to restart the stream

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cfg)
}

func NewPeakTracker() *PeakTracker {
	return &PeakTracker{
		state:    StateOff,
		isRising: false,
	}
}

func (pt *PeakTracker) CheckStateChange() {
	if pt.state != pt.lastState {
		pt.lastState = pt.state // Обновляем lastState после печати
		switch pt.state {
		case StateOff:
			fmt.Println("off")
			_, err := bluetoothChar.WriteWithoutResponse(getColor(cfg.Background))
			if err != nil {
				log.Fatalf("can't write characteristic: %s", err)
			}
		case StateOn:
			fmt.Println("on")
			_, err := bluetoothChar.WriteWithoutResponse(getColor(cfg.Color))
			if err != nil {
				log.Fatalf("can't write characteristic: %s", err)
			}
		}
	}
}

type ThresholdTracker struct {
	maxMovingAverage   *MovingAverage
	minMovingAverage   *MovingAverage
	threshold          float64
	silenceThreshold   float64       // New: Threshold to consider as silence
	silenceDuration    time.Duration // New: Duration to check for prolonged silence
	lastSoundTimestamp time.Time     // New: Timestamp of the last sound above silence threshold
}

func NewThresholdTracker(size int, silenceThreshold float64, silenceDuration time.Duration) *ThresholdTracker {
	return &ThresholdTracker{
		maxMovingAverage:   NewMovingAverage(size),
		minMovingAverage:   NewMovingAverage(size),
		silenceThreshold:   silenceThreshold,
		silenceDuration:    silenceDuration,
		lastSoundTimestamp: time.Now(),
	}
}

func (tt *ThresholdTracker) CheckAndResetThreshold() {
	fmt.Printf("Time since last sound: %v\n", time.Since(tt.lastSoundTimestamp))
	if time.Since(tt.lastSoundTimestamp) > tt.silenceDuration {
		// Reset the threshold to a default value or recalculate
		tt.maxMovingAverage.Reset() // Reset the moving averages
		tt.minMovingAverage.Reset()
		// Optionally set the threshold to a default value
		// tt.threshold = defaultThresholdValue
	}
}

func (tt *ThresholdTracker) Update(value float64) {
	// Update maxMovingAverage if the current value is higher than the current average max or if the window is not full yet.
	if value > tt.maxMovingAverage.Average() || !tt.maxMovingAverage.full {
		tt.maxMovingAverage.Add(value)
	}

	// Update minMovingAverage if the current value is lower than the current average min or if the window is not full yet.
	if value < tt.minMovingAverage.Average() || !tt.minMovingAverage.full && value > 1 {
		tt.minMovingAverage.Add(value)
	}
	fmt.Printf("Max: %f Min: %f Value: %f\n", tt.maxMovingAverage.Average(), tt.minMovingAverage.Average(), value)
	if value > tt.silenceThreshold {
		tt.lastSoundTimestamp = time.Now()
	}
	// Update the threshold based on the new max and min averages.
	tt.threshold = (tt.maxMovingAverage.Average() + tt.minMovingAverage.Average()) / 2
	tt.CheckAndResetThreshold()
}

func (ma *MovingAverage) Average() float64 {
	if ma.full {
		return ma.sum / float64(ma.size)
	}
	// If the window is not yet full, we divide by the actual number of elements
	return ma.sum / float64(ma.index)
}
func (pt *PeakTracker) UpdateWithThreshold(amplitude, threshold float64) {
	switch pt.state {
	case StateOff:
		if amplitude > threshold {
			pt.isRising = true
			pt.peakStartLevel = amplitude
		}
		if pt.isRising && amplitude < pt.peakStartLevel-threshold {
			pt.state = StateOn
			pt.peakEndLevel = amplitude
			pt.isRising = false
		}
	case StateOn:
		if amplitude < threshold {
			pt.state = StateOff
			pt.isRising = false
		}
	}
	pt.CheckStateChange()
}

func (pt *PeakTracker) UpdateWithDynamicThreshold(amplitude, threshold float64) {
	switch pt.state {
	case StateOff:
		if amplitude > threshold {
			pt.state = StateOn
		}
	case StateOn:
		if amplitude < threshold {
			pt.state = StateOff
		}
	}
	pt.CheckStateChange()
}

// type LowPassFilter struct {
// 	a        float64
// 	y, yPrev float64
// }

type MovingAverage struct {
	window []float64
	size   int
	index  int
	sum    float64
	full   bool
}

func NewMovingAverage(size int) *MovingAverage {
	return &MovingAverage{
		window: make([]float64, size),
		size:   size,
	}
}

func (ma *MovingAverage) Add(value float64) float64 {
	// Subtract the oldest value from the sum if the window is full.
	if ma.full {
		ma.sum -= ma.window[ma.index]
	}
	// Add the new value to the window and to the sum.
	ma.window[ma.index] = value
	ma.sum += value
	// Move the index forward and check if the window is full.
	ma.index = (ma.index + 1) % ma.size
	ma.full = ma.full || ma.index == 0
	// Return the average.
	return ma.sum / float64(ma.size)
}

func (ma *MovingAverage) Reset() {
	ma.window = make([]float64, ma.size)
	ma.index = 0
	ma.sum = 0
	ma.full = false
}

type LowPassFilter struct {
	y, yPrev float64
}

func NewLowPassFilter() *LowPassFilter {
	return &LowPassFilter{}
}

func (lpf *LowPassFilter) Apply(x float64) float64 {
	dt := 1.0 / cfg.SampleRate
	rc := 1.0 / (2 * math.Pi * cfg.CutoffFrequency)
	alpha := dt / (rc + dt)

	lpf.y = lpf.yPrev + alpha*(x-lpf.yPrev)
	lpf.yPrev = lpf.y
	return lpf.y
}

var adapter = bluetooth.DefaultAdapter

var bluetoothChar bluetooth.DeviceCharacteristic

func main() {
	thresholdTracker := NewThresholdTracker(40, 0.1, 100*time.Millisecond)
	peakTracker := NewPeakTracker()

	flag.Parse()

	fmt.Printf("Sample Rate: %v Hz\n", cfg.SampleRate)
	fmt.Printf("Threshold: %v\n", cfg.Threshold)
	fmt.Printf("Buffer Size: %v samples\n", cfg.BufferSize)
	fmt.Printf("Cutoff Frequency: %v Hz\n", cfg.CutoffFrequency)
	fmt.Printf("Delay: %v\n", cfg.Delay)

	adapter.Enable()
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
						fmt.Println("Characteristic written")
					}

				}
			}
		}
	})

	err := portaudio.Initialize()
	if err != nil {
		fmt.Println("PortAudio error:", err)
		return
	}

	devices, err := portaudio.Devices()
	if err != nil {
		fmt.Println("PortAudio error:", err)
		return
	}

	for i, device := range devices {
		fmt.Printf("[%d] %s\n", i, device.Name)
	}
	if err != nil {
		fmt.Println("PortAudio initialization failed:", err)
		return
	}
	defer portaudio.Terminate()

	inputChannels := make([]int16, cfg.BufferSize)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.HandleFunc("/api/config", GetConfigHandler)
	http.HandleFunc("/api/patch-config", PatchConfigHandler)
	// http.HandleFunc("/api/update-config", UpdateConfigHandler)
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	stream, err := portaudio.OpenDefaultStream(1, 0, cfg.SampleRate, int(cfg.BufferSize), inputChannels)

	if err != nil {
		fmt.Println("PortAudio stream error:", err)
		return
	}
	defer stream.Close()

	lowPassFilter := NewLowPassFilter()

	fmt.Println("Listening for sharp volume changes...")

	err = stream.Start()
	if err != nil {
		fmt.Println("PortAudio stream start error:", err)
		return
	}
	defer stream.Stop()

	ma := NewMovingAverage(11)
	for {
		err := stream.Read()
		// set delay
		time.Sleep(time.Duration(cfg.Delay) * time.Millisecond)
		if err != nil {
			fmt.Println("PortAudio stream read error:", err)
			return
		}

		sum := 0.0
		for i, sample := range inputChannels {
			// Применение фильтра к каждому сэмплу
			filteredSample := lowPassFilter.Apply(float64(sample))
			filteredIntSample := int16(filteredSample)
			inputChannels[i] = filteredIntSample // Обратное преобразование к int16 для вывода
			sum += math.Abs(float64(filteredIntSample))
		}

		// Далее в цикле, после вычисления текущей амплитуды:
		currentAmplitude := sum / float64(len(inputChannels))
		smoothedAmplitude := ma.Add(currentAmplitude)
		smoothedAmplitude = math.Max(0, smoothedAmplitude-cfg.Threshold)
		thresholdTracker.Update(smoothedAmplitude)
		// fmt.Printf("Current threshold: %f\n", thresholdTracker.threshold)
		// dynamicThreshold := thresholdTracker.threshold

		// Now use dynamicThreshold to determine if the current amplitude represents a peak.
		peakTracker.UpdateWithDynamicThreshold(smoothedAmplitude, thresholdTracker.threshold)
		fmt.Printf("Current state: %s Cutoff Frequency: %f", peakTracker.state, cfg.CutoffFrequency)
		// создать строку с амплитудой в начале и таким количеством звездочек, сколько амплитуда
		// (округленная до целого) делится на 1000
		// (т.е. если амплитуда 5000, то строка будет "5 *****")

		fmt.Println(smoothedAmplitude, strings.Repeat("*", int(smoothedAmplitude/15)))

	}
}

func getColor(color string) []byte {
	// sampleColorString := "ff0000"
	base := "6996060101"
	concate := base + color
	result, _ := hexStringToBytes(concate)
	return result
}

func hexStringToBytes(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("hex string must have an even length")
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}

	return b, nil
}
