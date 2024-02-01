package mr

import (
	"fmt"
	"hash/fnv"
	"log"
	"net/rpc"
	"os"
	"io/ioutil"
	"encoding/json"
	"path/filepath"
	"time"
	"math"
	"sort"
)

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }


// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// send an RPC to the coordinator asking for a task, then modify the coordinator to respond with the file name of a as-yet-unstarted map task
	// then, modify the worker to read that file and call the map function
	// uncomment to send the Example RPC to the coordinator.

	for {
			filename, nReduce, TaskNumber, Type, TemporaryFileLocations:= RetrieveTask()
			switch Type {
			case 0:
				println("Coordinator has no Tasks, sleep 1 sec")
				time.Sleep(time.Second * 1)
			case 1:	
				fmt.Printf("Retrieved Map Task %v, %v, %v \n", TaskNumber, nReduce, filename)
				// this probably needs to be task
				// this probably needs a return value, indicating the locations of the intermediate files 
				PerformMapTask(filename, nReduce, mapf, TaskNumber)
			case 2:
				fmt.Printf("Retrieved Reduce Task %v, %v, %v \n", TaskNumber, nReduce, filename)
				PerformReduceTask(TemporaryFileLocations, reducef, TaskNumber )
			case 3:
				break
			}
	}
	return
}

func PerformReduceTask(TemporaryFileLocations []string , reducef func(string, []string) string, ReduceTaskNumber int) {
    var ReducedTemporaryFileLocations []string

    for _, filename := range TemporaryFileLocations {
        if len(filename) > 0 {
            // Extract the numeric suffix from the filename
            numericSuffix, err := extractNumericSuffix(filename)
            if err == nil && numericSuffix == ReduceTaskNumber {
                ReducedTemporaryFileLocations = append(ReducedTemporaryFileLocations, filename)
            }
        }
    }

	// for _, filename := range ReducedTemporaryFileLocations {
	// 	fmt.Println(filename)
	// }


	var result []KeyValue
	for _, filePath := range ReducedTemporaryFileLocations {
		file, err := os.Open(filePath)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)

		// Assuming that each JSON file contains an array of KeyValues
		var kvs []KeyValue
		err = decoder.Decode(&kvs)
		if err != nil {
			log.Fatalf("error decoding JSON: %v", err)
		}

		// Append the decoded KeyValues to the result list
		result = append(result, kvs...)
	}
	sort.Sort(ByKey(result))

	// Assuming ReduceTaskNumber is defined somewhere in your code
	oname := fmt.Sprintf("mr-out-%d", ReduceTaskNumber)
	tmpFile, err := ioutil.TempFile("intermediate-files", oname)
	if err != nil {
		log.Fatalf("error creating temporary file: %v", err)
	}

	i := 0
	for i < len(result) {
		j := i + 1
		for j < len(result) && result[j].Key == result[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, result[k].Value)
		}
		output := reducef(result[i].Key, values)

		// This is the correct format for each line of Reduce output.
		fmt.Fprintf(tmpFile, "%v %v\n", result[i].Key, output)

		i = j
	}
	// Close the temporary file after writing
	tmpFile.Close()

	// Construct the destination path in the "output" directory
	destPath := filepath.Join(oname)

	// Rename the temporary file to the destination file
	err = os.Rename(tmpFile.Name(), destPath)
	if err != nil {
		log.Fatalf("error renaming file: %v", err)
		// Handle the error (e.g., log it) or simply discard the temporary file
		os.Remove(tmpFile.Name())
	}

	args := ConfirmationRequest{}
	args.TaskNumber = ReduceTaskNumber
	args.ConfirmationType = 1
	args.TemporaryFileLocations = TemporaryFileLocations
	// args.PID = os.Getpid()
	reply := ConfirmationResponse{}
	ok := call("Coordinator.TaskConfirmation", &args, &reply)
	if ok {
		return 	
	} else {
		return 
	}

}
func extractNumericSuffix(s string) (int, error) {
    var numericSuffix int
    var err error

    for i := len(s) - 1; i >= 0; i-- {
        if !isDigit(s[i]) {
            // Stop if we encounter a non-digit character
            break
        }

        digitValue := int(s[i] - '0')
        numericSuffix = digitValue*int(math.Pow10(len(s)-1-i)) + numericSuffix
    }

    return numericSuffix, err
}


func isDigit(char byte) bool {
    return char >= '0' && char <= '9'
}
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
func RetrieveTask() (string, int, int, int, []string) {

	// declare an argument structure.
	args := TaskRequest{}

	// fill in the argument(s).
	args.PID = os.Getpid()

	// declare a reply structure.
	reply := TaskResponse{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.TaskHandler", &args, &reply)
	if ok {
		return reply.FileName, reply.NReduce, reply.TaskNumber, reply.Type, reply.TemporaryFileLocations
	} else {
		fmt.Printf("call failed!\n")
		return "", -1, -1, -1, []string{"path1", "path2", "path3"}
	}
}

func createIntermediateFilesDirectory() error {
	directoryPath := "intermediate-files"

	// Check if the directory already exists
	if _, err := os.Stat(directoryPath); os.IsNotExist(err) {
		// Directory does not exist, create it
		err := os.Mkdir(directoryPath, 0755)
		if err != nil {
			return fmt.Errorf("error creating directory: %v", err)
		}
		fmt.Printf("Directory '%s' created.\n", directoryPath)
	}

	return nil
}
func PerformMapTask(filename string, nReduce int, mapf func(string, string) []KeyValue, MapTaskNumber int) {
	
	// retrieve list of kv pairs
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", filename)
	}
	file.Close()
	kva := mapf(filename, string(content))

	distributedKV := make([][]KeyValue, nReduce)
	

	// seperate kv pairs
	for i:= 0; i< len(kva); i++ {
		distributedKV[ihash(kva[i].Key)%nReduce] = append(distributedKV[ihash(kva[i].Key)%nReduce], kva[i])
	}
	
	pooperror := createIntermediateFilesDirectory()
	if pooperror != nil {
		fmt.Println(err)
	} else {

	}

	var PathNames []string
	for i := 0; i < nReduce; i++ {
		oname := fmt.Sprintf("mr-%d-%d", MapTaskNumber, i)
		tmpFile, err := ioutil.TempFile("intermediate-files", oname)
		if err != nil {
			log.Fatalf("error creating temporary file: %v", err)
		}
		enc := json.NewEncoder(tmpFile)
		err = enc.Encode(&distributedKV[i])
		// Close the temporary file after writing
		tmpFile.Close()
		// Construct the destination path in the "intermediatefiles" directory
		destPath := filepath.Join("intermediate-files", oname)
		PathNames = append(PathNames, destPath)
		// Rename the temporary file to the destination file
		err = os.Rename(tmpFile.Name(), destPath)
		if err != nil {
			log.Fatalf("error renaming file: %v", err)
			// Handle the error (e.g., log it) or simply discard the temporary file
			os.Remove(tmpFile.Name())
		}
	}
	args := ConfirmationRequest{}
	args.TaskNumber = MapTaskNumber
	args.ConfirmationType = 0
	args.TemporaryFileLocations = PathNames
	// args.PID = os.Getpid()
	reply := ConfirmationResponse{}
	ok := call("Coordinator.TaskConfirmation", &args, &reply)
	if ok {
		return 	
	} else {
		return 
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
