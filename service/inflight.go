package service

import "sync"

type inflightCounter struct {
	mu            sync.RWMutex
	userCounts    map[int]int
	channelCounts map[int]int
}

var requestsInflight = inflightCounter{
	userCounts:    make(map[int]int),
	channelCounts: make(map[int]int),
}

func BeginUserInflight(userID int) func() {
	return requestsInflight.begin(requestsInflight.userCounts, userID)
}

func BeginChannelInflight(channelID int) func() {
	return requestsInflight.begin(requestsInflight.channelCounts, channelID)
}

func GetUserInflightCount(userID int) int {
	return requestsInflight.get(requestsInflight.userCounts, userID)
}

func GetChannelInflightCount(channelID int) int {
	return requestsInflight.get(requestsInflight.channelCounts, channelID)
}

func (counter *inflightCounter) begin(counts map[int]int, id int) func() {
	if id <= 0 {
		return func() {}
	}
	counter.mu.Lock()
	counts[id]++
	counter.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			counter.mu.Lock()
			defer counter.mu.Unlock()
			if counts[id] <= 1 {
				delete(counts, id)
				return
			}
			counts[id]--
		})
	}
}

func (counter *inflightCounter) get(counts map[int]int, id int) int {
	if id <= 0 {
		return 0
	}
	counter.mu.RLock()
	defer counter.mu.RUnlock()
	return counts[id]
}
