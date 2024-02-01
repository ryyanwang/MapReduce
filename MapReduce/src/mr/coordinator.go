package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"time"
	"sync"
	// "fmt"
)
// STATUS
// IDLE = 0
// IN PROGRESS = 1
// COMPLETE = 2

type Task struct {
	Status int
}
type Coordinator struct {
	// one queue of tasks left to complete
	// one array of in process tasks

	MapTasks []Task
	MapTasksStatus bool
	ReduceTasksStatus bool
	ReduceTasks []Task
	FileNames []string
	TemporaryFileLocations [][]string
	NReduce int
	mu sync.Mutex
	MapStatus int


}

// -- RPC handlers for the worker to call.
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) TaskHandler(args *TaskRequest, reply *TaskResponse) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Are we done
	if c.MapTasksStatus && c.ReduceTasksStatus {
		reply.Type = 3
		return nil
	} else if c.MapTasksStatus == true {
		// Reduce Task
		ReduceTaskNumber := 0
		NoTasks := true
		for ReduceTaskNumber < len(c.ReduceTasks) {
			if c.ReduceTasks[ReduceTaskNumber].Status == 0 {
				c.ReduceTasks[ReduceTaskNumber].Status = 1
				NoTasks = false
				break
			}
			ReduceTaskNumber++
		}
		if NoTasks {
			reply.Type = 0
			return nil
		}

		go RunningReduceTask(c, ReduceTaskNumber)
		reply.Type = 2
		var TemporaryFileLocationsFlatArray []string
		for _, nestedArray := range c.TemporaryFileLocations {
			TemporaryFileLocationsFlatArray = append(TemporaryFileLocationsFlatArray, nestedArray...)
		}

		reply.TemporaryFileLocations = TemporaryFileLocationsFlatArray
		reply.FileName = ""
		reply.NReduce = c.NReduce
		reply.TaskNumber = ReduceTaskNumber
		// fmt.Printf("Distributed Reduce Task %v\n", reply.TaskNumber)
		return nil
	} else {
		// Map Task
		MapTaskNumber := 0
		NoTasks := true
		for MapTaskNumber < len(c.FileNames) {
			if c.MapTasks[MapTaskNumber].Status == 0 {
				c.MapTasks[MapTaskNumber].Status = 1
				NoTasks = false
				break
			}
			MapTaskNumber++
		}
		// No available tasks,
		if NoTasks {
			reply.Type = 0
			return nil
		}
		go RunningMapTask(c, MapTaskNumber)
		reply.Type = 1
		reply.FileName = c.FileNames[MapTaskNumber]
		reply.NReduce = c.NReduce
		reply.TaskNumber = MapTaskNumber
		return nil
	}
}

func (c *Coordinator) TaskConfirmation(request *ConfirmationRequest, reply *ConfirmationResponse) error {
	// have not handled pid, assumes no worker will fail
	c.mu.Lock()
	defer c.mu.Unlock()
	// Map Task Confirmation
	if request.ConfirmationType == 0 { 
		// fmt.Printf("Map Task Confirmation received from %v \n", request.TaskNumber)
		if c.MapTasks[request.TaskNumber].Status != 2 {
			c.MapTasks[request.TaskNumber].Status = 2
			c.TemporaryFileLocations[request.TaskNumber] = request.TemporaryFileLocations		
			MapTaskNumber := 0
			for MapTaskNumber < len(c.FileNames) {
				if (c.MapTasks[MapTaskNumber].Status != 2) {
					return nil
				}
				MapTaskNumber++
			}

			c.MapTasksStatus = true
			return nil
		// confirmation for reduction
		} else {
			// duplicate, do nothing
			return nil
		}
	} else {
		// fmt.Printf("Reduce Task Confirmation received from %v \n", request.TaskNumber)
		if c.ReduceTasks[request.TaskNumber].Status != 2 {
			c.ReduceTasks[request.TaskNumber].Status = 2
			// Print the status of all ReduceTasks
			// fmt.Printf("Status of ReduceTasks: %+v\n", c.ReduceTasks)
		
			for _, reduceTask := range c.ReduceTasks {
				if reduceTask.Status != 2 {
					return nil
				}
			}
		
			println("FINISHED TASKS")
			c.ReduceTasksStatus = true
			return nil
		// confirmation for reduction
		} else {
			// duplicate, do nothing
			return nil
		}
	}
	return nil
}

func RunningMapTask(c *Coordinator, MapTaskNumber int) {
	time.Sleep(10 * time.Second)
	if c.MapTasks[MapTaskNumber].Status == 1 {
		c.MapTasks[MapTaskNumber].Status = 0
	}
}

func RunningReduceTask(c *Coordinator, ReduceTaskNumber int) {
	time.Sleep(10 * time.Second)
	if c.ReduceTasks[ReduceTaskNumber].Status == 1 {
		c.ReduceTasks[ReduceTaskNumber].Status = 0
	}
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	return c.ReduceTasksStatus
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(FileNames []string, nReduce int) *Coordinator {
	c := Coordinator{
		FileNames: FileNames, // Initialize the global variable for FileNames
		NReduce: nReduce,
	}
	c.MapTasksStatus = false
	c.ReduceTasksStatus = false
	// Initialize MapTasks and ReduceTasks with default values
	c.MapTasks = make([]Task, len(c.FileNames)) // Initialize with a specific size
	for i := range c.MapTasks {
		c.MapTasks[i].Status = 0
	}
	c.TemporaryFileLocations = make([][]string, len(c.FileNames))
	c.ReduceTasks = make([]Task, nReduce)

	for i := 0; i < nReduce; i++ {
		c.ReduceTasks[i].Status = 0
	}
	println("Coordinator Running")
	c.server()
	return &c
}
