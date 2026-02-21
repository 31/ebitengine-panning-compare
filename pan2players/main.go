// Copyright 2020 The Ebiten Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	raudio "github.com/hajimehoshi/ebiten/v2/examples/resources/audio"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/images"
)

const (
	screenWidth  = 640
	screenHeight = 480
	sampleRate   = 48000
)

var ebitenImage *ebiten.Image

type Game struct {
	playerLeft  *audio.Player
	playerRight *audio.Player

	// panning goes from -1 to 1
	// -1: 100% left channel, 0% right channel
	// 0: 100% both channels
	// 1: 0% left channel, 100% right channel
	panning float64

	count int
	xpos  float64

	audioContext *audio.Context
}

func (g *Game) initAudioIfNeeded() {
	if g.playerLeft != nil {
		return
	}

	if g.audioContext == nil {
		g.audioContext = audio.NewContext(sampleRate)
	}

	oggSL, err := vorbis.DecodeF32(bytes.NewReader(raudio.Ragtime_ogg))
	if err != nil {
		log.Fatal(err)
	}
	leftStream := NewSingleChannelStream(audio.NewInfiniteLoop(oggSL, oggSL.Length()), true)
	g.playerLeft, err = g.audioContext.NewPlayerF32(leftStream)
	if err != nil {
		log.Fatal(err)
	}

	oggSR, err := vorbis.DecodeF32(bytes.NewReader(raudio.Ragtime_ogg))
	if err != nil {
		log.Fatal(err)
	}
	rightStream := NewSingleChannelStream(audio.NewInfiniteLoop(oggSR, oggSR.Length()), false)
	g.playerRight, err = g.audioContext.NewPlayerF32(rightStream)
	if err != nil {
		log.Fatal(err)
	}

	g.playerLeft.Play()
	g.playerRight.Play()
}

// time is within the 0 ... 1 range
func lerp(a, b, t float64) float64 {
	return a*(1-t) + b*t
}

func (g *Game) Update() error {
	g.count++
	r := float64(g.count) * ((1.0 / 60.0) * 2 * math.Pi) * 0.1 // full cycle every 10 seconds
	g.xpos = (float64(screenWidth) / 2) + math.Cos(r)*(float64(screenWidth)/2)
	g.panning = lerp(-1, 1, g.xpos/float64(screenWidth))

	// Initialize the audio after the panning is determined.
	g.initAudioIfNeeded()

	// Adjust each player's volume to achieve panning.
	// This uses the same linear scale as the original StereoPanStream.
	leftVolume := math.Min(g.panning*-1+1, 1)
	rightVolume := math.Min(g.panning+1, 1)
	g.playerLeft.SetVolume(leftVolume)
	g.playerRight.SetVolume(rightVolume)

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	pos := g.playerLeft.Position()
	msg := fmt.Sprintf(`TPS: %0.2f
This is an example using
stereo audio panning (2 players).
Current: %0.2f[s]
Panning: %.2f`, ebiten.ActualTPS(), float64(pos)/float64(time.Second), g.panning)
	ebitenutil.DebugPrint(screen, msg)

	// draw image to show where the sound is at related to the screen
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(g.xpos-float64(ebitenImage.Bounds().Dx()/2), screenHeight/2)
	screen.DrawImage(ebitenImage, op)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	// Decode an image from the image file's byte slice.
	img, _, err := image.Decode(bytes.NewReader(images.Ebiten_png))
	if err != nil {
		log.Fatal(err)
	}
	ebitenImage = ebiten.NewImageFromImage(img)

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Audio Panning Loop (Ebitengine Demo)")
	g := &Game{}
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

// SingleChannelStream passes audio for only one stereo channel, silencing the other.
// This lets each player be volume-controlled independently for panning.
type SingleChannelStream struct {
	io.ReadSeeker
	isLeft bool
}

func (s *SingleChannelStream) Read(p []byte) (int, error) {
	n, err := s.ReadSeeker.Read(p)
	// Each stereo float32 frame is 8 bytes: 4 bytes left + 4 bytes right.
	// Zero out the unwanted channel for every complete frame.
	for i := 0; i+8 <= n; i += 8 {
		if s.isLeft {
			// Silence the right channel.
			p[i+4] = 0
			p[i+5] = 0
			p[i+6] = 0
			p[i+7] = 0
		} else {
			// Silence the left channel.
			p[i] = 0
			p[i+1] = 0
			p[i+2] = 0
			p[i+3] = 0
		}
	}
	return n, err
}

func NewSingleChannelStream(src io.ReadSeeker, isLeft bool) *SingleChannelStream {
	return &SingleChannelStream{
		ReadSeeker: src,
		isLeft:     isLeft,
	}
}
