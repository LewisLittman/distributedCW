package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"uk.ac.bris.cs/gameoflife/util"

	//"math"
	"math/rand"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
	//	"errors"
	//	"flag"
	//	"fmt"
	"net"
	"time"
	//	"uk.ac.bris.cs/distributed2/secretstrings/stubs"
	//	"net/rpc"
)

//define global variables
var world [][]byte
var CompletedTurns int
var mutex = sync.Mutex{}
var addresses = make([]string, 4)
var noOfWorkers int
var reset = false
var shutdownmutex = sync.Mutex{}

type ResetStruct struct{}

//reset function locks the mutex
//sets the reset global variable to true
//returns the turns as a response to the distributor
//unlocks the mutex lock
func (s *ResetStruct) Reset(req stubs.ResetRequest, res *stubs.ResetResponse) (err error) {
	mutex.Lock()
	reset = true
	res.Turn = CompletedTurns
	mutex.Unlock()
	return
}

type PauseStruct struct{}

//prints its paused and the current turn
//holds the mutex lock so more evolutions can't continue
func (s *PauseStruct) Pause(req stubs.PauseRequest, res *stubs.PauseResponse) (err error) {
	println("Paused...")
	println("The current turn is: ", CompletedTurns)
	mutex.Lock()
	return
}

type PlayStruct struct{}

//unlocks the mutex the pause lock was holding to allow the evolution of the world to continue
func (s *PlayStruct) Play(req stubs.PauseRequest, res *stubs.PauseResponse) (err error) {
	println("Continuing...")
	mutex.Unlock()
	return
}

type BrokerShutdownStruct struct{}

//sets a mutex lock so workers cannot be called when theyre being shutdown
//loops through the addresses of the workers and dials each one
//sends a shutdown request to each and sets up an empty response
//once completed close the broker
func (s *BrokerShutdownStruct) BrokerShutdown(req stubs.BrokerShutdownRequest, res *stubs.BrokerShutdownResponse) (err error) {
	letter := req.Letter
	shutdownmutex.Lock()
	if letter == 'k' {
		for i := 0; i < noOfWorkers; i++ {
			fmt.Println(addresses[i] + ":8000")
			client, err := rpc.Dial("tcp", addresses[i]+":8000")
			if err != nil {
				panic(err)
			}
			defer client.Close()

			request := stubs.ShutdownRequest{}
			response := new(stubs.ShutdownResponse)
			_ = client.Call(stubs.ShutdownHandler, request, response)

		}
		//shutdownmutex.Unlock()
		println("Shutting Down Broker!")
		os.Exit(0)
		return
	}
	return
}

type KeyStruct struct{}

//locks the state
//returns the turns and world back to distributor
//unlocks the lock
func (s *KeyStruct) KeyPressInfo(_ stubs.KeyRequest, res *stubs.KeyResponse) (err error) {
	mutex.Lock()
	res.Turn = CompletedTurns
	res.World = world
	mutex.Unlock()
	return
}

type TickerStruct struct{}

//gets the image height and width from the distributor
//locks the mutex
//runs through each cell in the world calculating which cells are alive
//adds cells that are alive into a slice
//returns the number of alive cells and completed turns
//unlocks the mutex
func (s *TickerStruct) CalculateAliveCells(req stubs.TickerRequest, res *stubs.TickerResponse) (err error) {
	alive := []util.Cell{}
	ImageHeight := req.ImageHeight
	ImageWidth := req.ImageWidth

	mutex.Lock()
	for y := 0; y < ImageHeight; y++ {
		for x := 0; x < ImageWidth; x++ {
			if world[y][x] == 255 {
				alive = append(alive, util.Cell{x, y})
			}
		}
	}
	res.AliveCells = len(alive)
	res.Turn = CompletedTurns
	mutex.Unlock()
	return
}

type TurnsStruct struct {
	mutex sync.Mutex
}

func (s *TurnsStruct) Turn(req stubs.BrokerRequest, res *stubs.BrokerResponse) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	//initialises variables sent from distributor
	mutex.Lock()
	world = req.World
	turns := req.Turns

	imageHeight := req.ImageHeight
	imageWidth := req.ImageWidth
	CompletedTurns = 0
	mutex.Unlock()

	//initialises a slice of as many channels as there are number of workers
	channels := make([]chan [][]uint8, noOfWorkers)

	// Create channels in a loop
	for i := 0; i < noOfWorkers; i++ {
		channels[i] = make(chan [][]uint8)
	}

	divider := imageHeight / noOfWorkers

	//loops through turns
	//if the reset is true break out of distributor
	for t := 0; t < turns; t++ {
		mutex.Lock()
		shutdownmutex.Lock()
		if reset == true {
			reset = false
			mutex.Unlock()
			shutdownmutex.Unlock()
			break
		}
		mutex.Unlock()

		//set up a new world and loop through workers
		//get the start and end values of Y
		//if its the last worker give it the remaining part of the world
		var newWorld [][]byte
		for i := 0; i < noOfWorkers; i++ {
			//startY := int(math.Ceil(float64(imageHeight) / float64(noOfWorkers) * float64(i)))
			//endY := int(math.Ceil(float64(imageHeight) / float64(noOfWorkers) * float64(i+1)))
			startY := i * divider
			endY := (i + 1) * divider
			if i == noOfWorkers-1 {
				endY = imageHeight
			}

			//go function to call each worker with an RPC and pass it the needed data in the request
			go func(i int) {
				client, err := rpc.Dial("tcp", addresses[i]+":8000")
				if err != nil {
					panic(err)
				}

				defer client.Close()

				request2 := stubs.WorkerRequest{World: world, ImageHeight: imageHeight, ImageWidth: imageWidth, EndY: endY, StartY: startY}
				response2 := new(stubs.WorkerResponse)
				_ = client.Call(stubs.WorkHandler, request2, response2)

				// send the response down the channel
				channels[i] <- response2.NewWorld

			}(i)

		}

		//loop through number of workers and append each value to the newWorld variable
		for i := 0; i < noOfWorkers; i++ {
			newWorld = append(newWorld, <-channels[i]...)
		}
		//update the world and increment turns
		mutex.Lock()
		world = newWorld
		CompletedTurns++
		mutex.Unlock()
		shutdownmutex.Unlock()
	}
	//return the turns and world
	mutex.Lock()
	res.Turn = CompletedTurns
	res.World = world
	mutex.Unlock()
	return
}

//sets up the port of the broker
//flags to allow the IP of the worker nodes to be added to the addresses slice
//registers all structs
//sets up a listener to listen on the IP and port
func main() {
	pAddr := flag.String("port", "8030", "port to listen to")
	flag.IntVar(&noOfWorkers, "workers", 1, "number of workers")
	flag.StringVar(&addresses[0], "ip1", "0", "ip of worker 1")
	flag.StringVar(&addresses[1], "ip2", "0", "ip of worker 2")
	flag.StringVar(&addresses[2], "ip3", "0", "ip of worker 3")
	flag.StringVar(&addresses[3], "ip4", "0", "ip of worker 4")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&TurnsStruct{})
	rpc.Register(&TickerStruct{})
	rpc.Register(&KeyStruct{})
	rpc.Register(&BrokerShutdownStruct{})
	rpc.Register(&PauseStruct{})
	rpc.Register(&PlayStruct{})
	rpc.Register(&ResetStruct{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
