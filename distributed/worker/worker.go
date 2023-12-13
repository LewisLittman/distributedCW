package main

import (
	"flag"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

//Function to create a new GOL world. Initialises the world then loops through the array making the cells
func createNewWorld(height, width int) [][]byte {
	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}
	return newWorld
}

//Function to calculate the alive neighbours of a cell. Checks the adjacent cells and increments
//a sum variable if the value of the cell is 255 (alive)
func calculateAliveNeighbours(ImageHeight, ImageWidth int, x int, y int, world [][]byte) int {
	sum := 0

	for j := -1; j <= 1; j++ {
		for l := -1; l <= 1; l++ {
			ny := y + j
			nx := x + l

			if j == 0 && l == 0 {
				continue
			}
			if nx < 0 {
				nx = ImageWidth - 1
			}
			if nx >= ImageWidth {
				nx = 0
			}
			if ny < 0 {
				ny = ImageHeight - 1
			}
			if ny >= ImageHeight {
				ny = 0
			}
			if world[ny][nx] == 255 {
				sum++
			}
		}
	}

	return sum
}

//This function is used to complete a turn in GOL
//We calculate a height based upon the start and end Y values then create a new world corresponding to this new height
//We then loop through the height and the width and call the calculate alive neighbours function on each cell
//We then check if the current cell is alive and compare it to the rules of GOL
//If it needs to be dead we call an event to flip the cell from alive to dead
//Else if the cell was dead to start we check if it should become alive using the GOL rules
//If so, we flip the cell to alive with the event, else we do nothing. Then we return the new world
func completeTurn(ImageHeight, ImageWidth, startY, endY int, world [][]byte) [][]byte {
	height := endY - startY
	newWorld := createNewWorld(height, ImageWidth)
	for y := startY; y < endY; y++ {
		for x := 0; x < ImageWidth; x++ {
			aliveNeighbours := calculateAliveNeighbours(ImageHeight, ImageWidth, x, y, world)
			if world[y][x] == 255 {
				if aliveNeighbours == 2 || aliveNeighbours == 3 {
					newWorld[y-startY][x] = 255
				}
			} else {
				if aliveNeighbours == 3 {
					newWorld[y-startY][x] = 255
				}
			}
		}
	}
	return newWorld
}

type WorkStruct struct{}

//gather all info sent from broker in variables
//set the response back to the response of the complete turn function
func (w *WorkStruct) Work(request stubs.WorkerRequest, response *stubs.WorkerResponse) (err error) {
	if err != nil {
		panic(err)
	}
	world := request.World
	ImageHeight := request.ImageHeight
	ImageWidth := request.ImageWidth
	startY := request.StartY
	endY := request.EndY
	response.NewWorld = completeTurn(ImageHeight, ImageWidth, startY, endY, world)

	return
}

type ShutdownStruct struct{}

//shut down the worker
func (s *ShutdownStruct) Shutdown(request stubs.ShutdownRequest, response *stubs.ShutdownResponse) (err error) {
	println("Shutting Down Worker!")
	os.Exit(0)
	return
}

//sets up the port of the worker
//registers all structs
//sets up a listener to listen on the IP and port
func main() {
	pAddr := flag.String("port", "8000", "port to listen to")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&WorkStruct{})
	rpc.Register(&ShutdownStruct{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
