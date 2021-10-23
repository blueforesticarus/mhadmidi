package main

import (
	"fmt"
	"log"
	"math"
	"os"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/xthexder/go-jack"
)

var channels int = 2

var PortsIn []*jack.Port
var PortsOut []*jack.Port

func process(nframes uint32) int {
	samples := PortsOut[0].GetBuffer(nframes)
	R := realtime.Raster(len(samples))
	for i := range samples {
		if i < R.Len() {
			samples[i] = jack.AudioSample(R.Data[i])
		} else {
			samples[i] = 0
		}
	}
	return 0
}

var realtime *RealtimeAudio

type RealtimeAudio struct {
	source Rasterable
	offset int
}

func (self *RealtimeAudio) Raster(length int) Sample {
	ret := self.source.Raster(self.offset, length)
	self.offset += length
	return ret
}

func LoadSample(filename string, samplerate int) Sample {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	resampled := beep.Resample(4, format.SampleRate, beep.SampleRate(samplerate), streamer)

	out := make([]float32, 0, streamer.Len()*int(resampled.Ratio()))
	samples := make([][2]float64, 100)
	var off int
	for {
		n, ok := resampled.Stream(samples)
		if !ok {
			break
		}
		for i := 0; i < n; i++ {
			out = append(out, float32(samples[i][0]))
		}
		off += n
	}

	return Sample{
		Data:    out,
		Storage: -1,
	}
}

func QuantBPM(samples int, bpm int, rate int) int {
	beat := float64(rate) / (float64(bpm) / 60)
	n := math.Round(float64(samples) / beat)
	return int(math.Round(beat * n))
}

func test_looping(client *jack.Client) {
	var sample1, sample2 Sample
	var loop1, loop2 Loop
	var master MultiLoop

	rate := client.GetSampleRate()
	sample1 = LoadSample("guitar.mp3", int(rate))
	sample2 = LoadSample("drum.mp3", int(rate))

	q := sample2.Len()
	q = QuantBPM(q, 109, int(rate))

	l := sample1.Len() / q
	loop1 = Loop{
		Period: l * q,
		sample: sample1,
		Cut2:   sample1.Len(),
		Start:  int(.1 * float32(rate)),
	}

	loop2 = Loop{
		Period: q,
		sample: sample2,
		Cut2:   sample2.Len(),
	}

	master.samples = []Rasterable{makeCache(loop1), makeCache(loop2)}

	realtime = &RealtimeAudio{
		source: master,
	}
}

func main() {

	client, status := jack.ClientOpen("Go Passthrough", jack.NoStartServer)
	if status != 0 {
		fmt.Println("Status:", jack.StrError(status))
		return
	}
	defer client.Close()

	test_looping(client)

	if code := client.SetProcessCallback(process); code != 0 {
		fmt.Println("Failed to set process callback:", jack.StrError(code))
		return
	}
	shutdown := make(chan struct{})
	client.OnShutdown(func() {
		fmt.Println("Shutting down")
		close(shutdown)
	})

	if code := client.Activate(); code != 0 {
		fmt.Println("Failed to activate client:", jack.StrError(code))
		return
	}

	for i := 0; i < channels; i++ {
		portIn := client.PortRegister(fmt.Sprintf("in_%d", i), jack.DEFAULT_AUDIO_TYPE, jack.PortIsInput, 0)
		PortsIn = append(PortsIn, portIn)
	}
	for i := 0; i < channels; i++ {
		portOut := client.PortRegister(fmt.Sprintf("out_%d", i), jack.DEFAULT_AUDIO_TYPE, jack.PortIsOutput, 0)
		PortsOut = append(PortsOut, portOut)
	}

	fmt.Println(client.GetName())

	<-shutdown
}
