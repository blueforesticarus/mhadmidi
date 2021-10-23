package main

import (
	"github.com/pkg/math"
)

// More absolute clowning on the part of go developers
// There is no standard library way to do modulus
func mod(a, b int) int {
	return (a%b + b) % b
}

/*
This file defines the core operational model for the looper

It also implictly defines the save format

Building off the failures of goontunes, it also will define correctness checks on all structures

Requirements:
	- do not loose data on overdubbing
*/

type AudioSample = float32

type Rasterable interface {
	Raster(offset, length int) Sample
	Len() int
}

//pointers to samples are never allowed
type Sample struct {
	Data    []AudioSample //fuck it 32 bit audio
	Storage int           //the root audio block that this slice is from
	//-1 for unmanaged
}

func (self Sample) Raster(offset, length int) Sample {
	var l = len(self.Data)
	i1 := math.Min(offset, l)
	i2 := math.Min(offset+length, l)

	self.Data = self.Data[i1:i2]
	return self //not a pointer
}

func (self Sample) Len() int {
	return len(self.Data)
}

//TODO trim
//problem is that changing beginning trim will change everything down the line
/* Options:
1. give sample an index clause
2. have raster return an offset int
3. put cut/trim params into any thing that would need to know about trim changes
*/
type SmartTrim struct {
}

type Loop struct {
	// a loop is a sample, togeather with cut and loop points
	// it is the data from the point I press the pedal
	// to the point I press it again
	// but unlike a looper pedal, we dont overdub
	// the whole track is saved
	// we have the option to raster a loop later on
	sample Rasterable

	// we include start and stop for trim
	Start  int //canonical begining of loop
	Period int //length of the loop

	//somehow these should be external
	Cut1 int
	Cut2 int
}

func (self Loop) Raster(offset, length int) Sample {
	var src = self.sample.Raster(self.Cut1, self.Cut2)
	start := self.Start - self.Cut1 //adjust index for trimming
	src_l := len(src.Data)

	/*
		TODO: efficiency could be alot better
			1. copy data when requested length is longer than period
			2. when possible, return a slice of src without copying and allocing
	*/

	//its a loop so it can provide any requested amount of audio
	var data = make([]AudioSample, length)
	for i := 0; i < length; i++ {
		/*
			i + offset is the position in the audio stream that was requested
			i + offset + start corrosponds to this location in src
			we want to iterate over all the equivilent values in src with modulo
			we use the mod function we define in order to always get the positive modulo
		*/
		L := mod(i+offset+start, self.Period) // The first matching index
		for ii := L; ii < src_l; ii += self.Period {
			data[i] += src.Data[ii]
		}
	}

	return Sample{
		Data:    data,
		Storage: -1,
	}
}

func (self Loop) Len() int {
	return self.Period
}

type Sequence struct {
	samples   []Rasterable
	positions [][]int
	length    int
}

func (self Sequence) Len() int {
	return self.length
}

func (self Sequence) Raster(offset, length int) Sample {
	panic("unimplemented")
}

type MultiLoop struct {
	samples []Rasterable //might not be loop, could be something wrapping loop
}

func (self MultiLoop) Len() int {
	min := self.samples[0].Len()
	for i := 1; i < len(self.samples); i++ {
		l := self.samples[i].Len()
		if min > l {
			min = l
		}
	}
	return min
}

func (self MultiLoop) Raster(offset, length int) Sample {
	//multiloop can provide any amount of audio
	var data = make([]AudioSample, length)

	for _, s := range self.samples {
		s_len := s.Len()
		s := s.Raster(0, s_len)

		for i := 0; i < length; i++ {
			ii := mod(i+offset, s_len)
			data[i] += s.Data[ii]
		}
	}
	return Sample{
		Data:    data,
		Storage: -1,
	}
}

type Cache struct {
	sample Rasterable
	cache  Sample
}

func (self Cache) Raster(offset, length int) Sample {
	return self.cache.Raster(offset, length) //not a pointer
}

func (self Cache) Len() int {
	return self.cache.Len()
}

func (self *Cache) Cache() {
	self.cache = self.sample.Raster(0, self.sample.Len())
}

func makeCache(s Rasterable) Cache {
	c := Cache{
		sample: s,
	}
	c.Cache()
	return c
}
