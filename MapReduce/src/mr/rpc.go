package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type TaskRequest struct {
	PID int
	X int
}

type TaskResponse struct {
	FileName string
	NReduce int
	TaskNumber int
	// TYPE:
	// 0: IDLE 
	// 1: MAP
	// 2: REDUCE
	// 3: Finished
	Type int
	TemporaryFileLocations []string
}

type ConfirmationRequest struct {
	TaskNumber int
	TemporaryFileLocations []string
	// TYPE:
	// 0: Map Task 
	// 1: Reduction Task
	ConfirmationType int
}

type ConfirmationResponse struct {
	Status string
}
// Add your RPC definitions here.

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
