package delayq

import (
	"container/heap"
	"sync"
	"time"
)

// Item is the delay queue item
type Item struct {
	value interface{}
	end   time.Time
	index int
}

type itemHeap []*Item

func (h itemHeap) Len() int {
	return len(h)
}

func (h itemHeap) Less(i, j int) bool {
	return h[i].end.Before(h[j].end)
}

func (h itemHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *itemHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*Item)
	item.index = n
	*h = append(*h, item)
}

func (h *itemHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[:n-1]
	return item
}

type outQueue struct {
	mu   sync.Mutex
	out  chan interface{}
	data []interface{}
}

func newOutQueue() *outQueue {
	return &outQueue{out: make(chan interface{}, 1)}
}

func (o *outQueue) close() {
	if o.out != nil {
		close(o.out)
		o.out = nil
	}
}

// push adds an item to the queue
func (o *outQueue) push(item interface{}) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.data = append(o.data, item)
	o.shift()
}

// shift moves the next available item from the queue into the out channel
// if the out channel is free the function returns immediately.
// the out channel value is returned to the user. must be locked by the caller
func (o *outQueue) shift() {
	if len(o.data) > 0 {
		select {
		case o.out <- o.data[0]:
			o.data = o.data[1:]
		default:
		}
	}
}

// pop returns the element and the status of the queue (closed or not)
func (o *outQueue) pop() (interface{}, bool) {
	item, ok := <-o.out
	if ok {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.shift()
		return item, false
	}

	return nil, true
}

// DelayQueue is a synchronized delay queue which is similar
// or a specialized form of a priority queue. The
// queue orders the elements based on their delay time.
// Queue internally uses container/heap and a goroutine.
// the goroutine operates on the heap depends on the calls
// made by the producer or consumer
type DelayQueue struct {
	itemHeap itemHeap
	wg       sync.WaitGroup
	in       chan interface{}
	outq     *outQueue
	stop     chan struct{}
}

// NewDelayQueue initialize a new delay queue and kick start
// the go routine. cap is the number of elements the input queue can
// hold. The output queue is unbounded, if the expired items are not read
// by the consumer then the items are placed in an array and that array
// can grow
func NewDelayQueue(cap int) *DelayQueue {
	dq := &DelayQueue{
		itemHeap: make([]*Item, 0, cap),
		in:       make(chan interface{}, 1),
		outq:     newOutQueue(),
		stop:     make(chan struct{}),
	}

	heap.Init(&dq.itemHeap)
	dq.wg.Add(1)
	go dq.run()

	return dq
}

// Close Close the queue and stops the goroutine
func (dq *DelayQueue) Close() {
	close(dq.stop)
	dq.outq.close()
	dq.wg.Wait()
}

func (dq *DelayQueue) run() {
	defer dq.wg.Done()

	timer := time.NewTimer(0)
	defer timer.Stop()

	nextTimer := func() {
		if dq.itemHeap.Len() > 0 {
			item := dq.itemHeap[0]
			timer = time.NewTimer(item.end.Sub(time.Now()))
		}
	}

	for {
		select {
		case item := <-dq.in:
			heap.Push(&dq.itemHeap, item)
			if len(dq.itemHeap) == 1 || item.(*Item).index == 0 {
				nextTimer()
			}
		case <-timer.C:
			if len(dq.itemHeap) > 0 {
				dq.outq.push(heap.Pop(&dq.itemHeap))
				nextTimer()
			}
		case <-dq.stop:
			return
		}
	}
}

// Put add an element into the queue. The priority is calculated
// based on the delay input.
func (dq *DelayQueue) Put(x interface{}, delay time.Duration) {
	item := &Item{value: x, end: time.Now().Add(delay)}
	dq.in <- item
}

// Get Returns the element whose delay is expired and the queue status(closed or not).
// The function waits till the first element is expired or the queue is closed.
func (dq *DelayQueue) Get() (interface{}, bool) {
	item, closed := dq.outq.pop()
	if closed {
		return nil, true
	}
	return item.(*Item).value, false
}
