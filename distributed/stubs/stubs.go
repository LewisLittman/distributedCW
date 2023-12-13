package stubs

var TurnsHandler = "TurnsStruct.Turn"
var WorkHandler = "WorkStruct.Work"
var TickerHandler = "TickerStruct.CalculateAliveCells"
var KeyHandler = "KeyStruct.KeyPressInfo"
var ShutdownHandler = "ShutdownStruct.Shutdown"
var BrokerShutdownHandler = "BrokerShutdownStruct.BrokerShutdown"
var PauseHandler = "PauseStruct.Pause"
var PlayHandler = "PlayStruct.Play"
var ResetHandler = "ResetStruct.Reset"

type BrokerResponse struct {
	World [][]byte
	Turn  int
}

type BrokerRequest struct {
	World       [][]byte
	Turns       int
	Threads     int
	ImageHeight int
	ImageWidth  int
}

type WorkerResponse struct {
	NewWorld [][]byte
}

type WorkerRequest struct {
	World       [][]byte
	ImageHeight int
	ImageWidth  int
	StartY      int
	EndY        int
}

type TickerRequest struct {
	ImageHeight int
	ImageWidth  int
}
type TickerResponse struct {
	AliveCells int
	Turn       int
}

type KeyResponse struct {
	Turn  int
	World [][]byte
}

type KeyRequest struct {
}
type ShutdownRequest struct {
}
type ShutdownResponse struct {
}
type BrokerShutdownRequest struct {
	Letter rune
}
type BrokerShutdownResponse struct {
}
type PauseRequest struct {
}
type PauseResponse struct {
}
type PlayRequest struct {
}
type PlayResponse struct {
}
type ResetRequest struct {
}
type ResetResponse struct {
	Turn int
}
