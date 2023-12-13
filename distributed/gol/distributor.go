package gol

import "C"
import (
	"fmt"
	"net/rpc"
	"os"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

//holds the IP address of the broker
const brokerAddr = "3.81.69.176:8030"

var PausedCheck = false

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

//Function to create a new GOL world. Initialises the world then loops through the array making the cells
func createNewWorld(height, width int) [][]byte {
	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}
	return newWorld
}

//function to calculate all the alive cells in the current state. We use for loops to loop through the height
//of the image then the width. Then using the width and height values, we check if each cell is alive or dead
//if the cell is alive we append it to the slice with its x and y values. The slice is then returned
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	alive := []util.Cell{}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				alive = append(alive, util.Cell{x, y})
			}
		}
	}

	return alive
}

//Function to run the ticker
//we use an RPC to send the image height, image width to the broker
//we then run the alive cell count down the events channel with alive cells and turn value that has been sent back as a response from the broker
func runTicker(ticker *time.Ticker, p Params, c distributorChannels) {
	for {
		select {
		case <-ticker.C:

			client, _ := rpc.Dial("tcp", brokerAddr)
			defer client.Close()

			request := stubs.TickerRequest{ImageHeight: p.ImageHeight, ImageWidth: p.ImageWidth}
			response := new(stubs.TickerResponse)
			_ = client.Call(stubs.TickerHandler, request, response)

			c.events <- AliveCellsCount{response.Turn, response.AliveCells}
		}

	}
}

//key press function to handle key presses
//infinite for loop so always checking for keypress
func keyPress(p Params, c distributorChannels) {
	for {

		//if the key is s
		//rpc dial the broker
		//set the turn and world variables to the response sent back by the broker
		//call the pmgOutput with these new variables to output a pgm of the world
		key := <-c.keyPresses
		switch key {
		case 's':
			client, _ := rpc.Dial("tcp", brokerAddr)
			defer client.Close()

			request := stubs.KeyRequest{}
			response := new(stubs.KeyResponse)
			err := client.Call(stubs.KeyHandler, request, response)
			if err != nil {
				panic(err)
			}

			turn := response.Turn
			world := response.World
			pgmOutput(p, c, world, turn)

		// if the key press is k
		//rpc call the broker with empty request and response
		//set the turn and world variables to the response sent back by the broker
		//call the pmgOutput with these new variables to output a pgm of the world
		//then send another request to the brokerShutdown struct
		//check it's idle if it is shut down the distributor
		case 'k':

			client, _ := rpc.Dial("tcp", brokerAddr)
			defer client.Close()

			request := stubs.KeyRequest{}
			response := new(stubs.KeyResponse)
			_ = client.Call(stubs.KeyHandler, request, response)

			turn := response.Turn
			pgmOutput(p, c, response.World, turn)

			request1 := stubs.BrokerShutdownRequest{Letter: 'k'}
			response1 := new(stubs.BrokerShutdownRequest)
			_ = client.Call(stubs.BrokerShutdownHandler, request1, response1)

			c.ioCommand <- ioCheckIdle
			<-c.ioIdle

			c.events <- StateChange{turn, Quitting}
			close(c.events)
			println("Shutting Down Distributor!")
			os.Exit(0)

		//check if the global variable pausecheck is false
		//if it isn't dial the broker with an rpc and pause request
		//set pause check to true
		//else if it is true when p is pressed
		//dial the broker with a play request
		//set the pause check back to false
		case 'p':
			if PausedCheck == false {
				client, _ := rpc.Dial("tcp", brokerAddr)
				defer client.Close()
				request := stubs.PauseRequest{}
				response := new(stubs.PauseResponse)
				err := client.Call(stubs.PauseHandler, request, response)
				if err != nil {
					panic(err)
				}
				PausedCheck = true
				fmt.Println("Paused...")
			} else {
				client, _ := rpc.Dial("tcp", brokerAddr)
				defer client.Close()
				request1 := stubs.PlayRequest{}
				response1 := new(stubs.PlayResponse)
				err := client.Call(stubs.PlayHandler, request1, response1)
				if err != nil {
					panic(err)
				}
				PausedCheck = false
				fmt.Println("Unpaused...")
			}

		//dial the broker with an rpc call on the reset request struct
		//get the current turn back from the response and save it in the turn vairable
		//check if the program is idle
		//quit the distributor
		case 'q':
			client, _ := rpc.Dial("tcp", brokerAddr)
			defer client.Close()
			request := stubs.ResetRequest{}
			response := new(stubs.ResetResponse)
			err := client.Call(stubs.ResetHandler, request, response)
			if err != nil {
				panic(err)
			}
			turn := response.Turn
			c.ioCommand <- ioCheckIdle
			<-c.ioIdle
			c.events <- StateChange{turn, Quitting}
			close(c.events)
			os.Exit(0)
		}

	}
}

//this is a function to output the state of the world as a PGM image
//we create a filename based on the image height, width and the turn given
//we send the ioOutput down the command channel telling it an output is occuring and the filename down the filename channel
//we then loop through each cell in the world and sed it down the iOutput channel
//we finally call the event which displays the turn and the name of the file
func pgmOutput(p Params, c distributorChannels, world [][]byte, turn int) {
	outFilename := fmt.Sprint(p.ImageHeight) + "x" + fmt.Sprint(p.ImageWidth) + "x" + fmt.Sprint(turn)
	c.ioCommand <- ioOutput
	c.ioFilename <- outFilename
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- world[i][j]
		}
	}
	c.events <- ImageOutputComplete{turn, outFilename}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	turn := 0

	//we create an initial world that is as big as the image
	//we then loop through the world and send inital input into the world
	//we check if the cell input is alive, if so we flip the cell to make it alive
	inFilename := fmt.Sprint(p.ImageHeight) + "x" + fmt.Sprint(p.ImageWidth)
	c.ioCommand <- ioInput
	c.ioFilename <- inFilename
	world := createNewWorld(p.ImageHeight, p.ImageWidth)
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			world[i][j] = <-c.ioInput
			if world[i][j] == 255 {
				c.events <- CellFlipped{turn, util.Cell{j, i}}
			}
		}
	}

	//create a new ticker and run the ticker and keypress go routines
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	go runTicker(ticker, p, c)
	go keyPress(p, c)

	//dial the broker and pass in the appropriate data needed to calculate the next state of the world into the request
	//set a response with data needed back from the broker
	client, _ := rpc.Dial("tcp", brokerAddr)
	defer client.Close()

	request := stubs.BrokerRequest{World: world, Turns: p.Turns, Threads: p.Threads, ImageHeight: p.ImageHeight, ImageWidth: p.ImageWidth}
	response := new(stubs.BrokerResponse)
	client.Call(stubs.TurnsHandler, request, response)

	//set the values received back as variables
	world = response.World
	turn = response.Turn

	//if the world isn't nill send the final turn complete down the events channel and output a pgm of the world
	if world != nil {
		c.events <- FinalTurnComplete{turn, calculateAliveCells(p, world)}
		pgmOutput(p, c, world, turn)
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	//shut down the distributor
	close(c.events)
}
