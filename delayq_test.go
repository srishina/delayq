package delayq

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBasic(t *testing.T) {
	q := NewDelayQueue(12)
	data := "Hello world!"
	q.Put(data, 400*time.Millisecond)

	done := make(chan struct{})

	go func() {
		defer close(done)
		d, _ := q.Get()
		assert.Equal(t, data, d)
	}()

	select {
	case <-time.After(time.Second):
		t.Error("should not timeout")
	case <-done:
	}
	q.Close()
}

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func TestQRandomString(t *testing.T) {

	min := int(2)
	max := int(10)

	stop := make(chan struct{}, 1)
	wg := sync.WaitGroup{}
	wg.Add(2)

	nitems := 64
	pq := NewDelayQueue(nitems)

	go func() {
		defer wg.Done()
		for i := 0; i < nitems; i++ {
			randomStrLen := rand.Intn(12) + 4
			d := rand.Intn(max-min) + min
			pq.Put(randomString(randomStrLen), time.Duration(d)*time.Second)
		}
	}()

	receivedItems := 0

	go func() {
		defer wg.Done()
		for {
			_, closed := pq.Get()
			if closed {
				return
			}
			receivedItems++
			if receivedItems == nitems {
				close(stop)
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		<-stop
		pq.Close()
		wg.Wait()
	}()

	select {
	case <-time.After(11 * time.Second):
		t.Error("should not timeout")
	case <-done:
		assert.Equal(t, nitems, receivedItems)
	}
}
