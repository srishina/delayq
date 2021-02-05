# delayq
Delayed queue in golang

An example

``` golang
package main

import (
	"fmt"
	"time"

	"github.com/srishina/delayq"
)

const (
	delayDuration = 4 * time.Second
)

func main() {
	delayedq := delayq.NewDelayQueue(12)
	defer delayedq.Close()

	testItem := "Hello world!"
	// place an item into the queue with the delay duration
	delayedq.Put(testItem, delayDuration)

	// the input items are delayed
	done := make(chan struct{})
	go func() {
		defer close(done)
		item, _ := delayedq.Get()
		fmt.Println("Delayed item received, value: ", item)
	}()

	fmt.Println("Wait till the delayed item is received...")

	select {
	case <-time.After(delayDuration + time.Second):
		fmt.Println("error: shoud not timeout")
	case <-done:
		fmt.Println("Finished...")
	}
}
```

LICENSE
-------
MIT