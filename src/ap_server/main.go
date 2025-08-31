package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

/*
This is what the applications servers are running.
use `docker pull rich329/pivotproxy:latest` to pull locally and run
*/
const (
	port = "8989"
)

func maxArea(height []int) int {
	i, j := 0, len(height)-1
	maxA := 0

	for i < j {
		// compute area with current pair
		h := height[i]
		if height[j] < h {
			h = height[j]
		}
		area := (j - i) * h
		if area > maxA {
			maxA = area
		}

		// move the pointer at the shorter line
		if height[i] < height[j] {
			i++
		} else {
			j--
		}
	}
	return maxA
}

func handler(w http.ResponseWriter, r *http.Request) {
	// Write a 200 OK status (default if you write something)
	fmt.Println("basic <> request has reached sever <<-->>")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Hello, World!") // optional response body
}
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// Write a 200 OK status (default if you write something)
	fmt.Println("health check request has reached server <<-->>")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

type IntArray struct {
	Values []int `json:"values"`
}

func processWork(w http.ResponseWriter, r *http.Request) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("(1)Failed to parse request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	fmt.Println("Received request body:", string(body))
	var intArray IntArray
	if err := json.Unmarshal(body, &intArray); err != nil {
		http.Error(w, fmt.Sprintf("(2)Failed to parse request body: %v", err), http.StatusBadRequest)
		return
	}
	answ := maxArea(intArray.Values)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"max_area": answ,
		"inputSize": len(intArray.Values)})
}

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/process", processWork)

	// Start server on port 8080
	fmt.Println("The http server is running on port: ", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
