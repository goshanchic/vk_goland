package main

import (
	"fmt"
	"sort"
	"sync"
)

func RunPipeline(cmds ...cmd) {
	wg := &sync.WaitGroup{}
	var in chan interface{}
	for _, c := range cmds {
		out := make(chan interface{})
		wg.Add(1)
		go func(c cmd, in, out chan interface{}) {
			defer wg.Done()
			defer close(out)
			c(in, out)
		}(c, in, out)
		in = out
	}
	wg.Wait()
}

func SelectUsers(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	var mu sync.Mutex
	seen := make(map[uint64]struct{})

	for emailRaw := range in {
		email, ok := emailRaw.(string)
		if !ok {
			continue
		}
		wg.Add(1)
		go func(email string) {
			defer wg.Done()
			user := GetUser(email)
			mu.Lock()
			if _, exists := seen[user.ID]; !exists {
				seen[user.ID] = struct{}{}
				mu.Unlock()
				out <- user
			} else {
				mu.Unlock()
			}
		}(email)
	}
	wg.Wait()
}

func SelectMessages(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	batchSize := GetMessagesMaxUsersBatch
	if batchSize <= 0 {
		batchSize = 1
	}
	batch := make([]User, 0, batchSize)

	flush := func(users []User) {
		wg.Add(1)
		go func(users []User) {
			defer wg.Done()
			msgs, err := GetMessages(users...)
			if err != nil {
				return
			}
			for _, msg := range msgs {
				out <- msg
			}
		}(users)
	}

	for userRaw := range in {
		user, ok := userRaw.(User)
		if !ok {
			continue
		}
		batch = append(batch, user)
		if len(batch) == batchSize {
			flush(batch)
			batch = make([]User, 0, batchSize)
		}
	}
	if len(batch) > 0 {
		flush(batch)
	}
	wg.Wait()
}

func CheckSpam(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	limit := HasSpamMaxAsyncRequests
	if limit <= 0 {
		limit = 1
	}
	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for msgRaw := range in {
				msgID, ok := msgRaw.(MsgID)
				if !ok {
					continue
				}
				hasSpam, err := HasSpam(msgID)
				if err != nil {
					continue
				}
				out <- MsgData{ID: msgID, HasSpam: hasSpam}
			}
		}()
	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	var msgs []MsgData
	for raw := range in {
		msg, ok := raw.(MsgData)
		if !ok {
			continue
		}
		msgs = append(msgs, msg)
	}

	sort.Slice(msgs, func(i, j int) bool {
		if msgs[i].HasSpam != msgs[j].HasSpam {
			return msgs[i].HasSpam
		}
		return msgs[i].ID < msgs[j].ID
	})

	for _, msg := range msgs {
		out <- fmt.Sprintf("%t %d", msg.HasSpam, msg.ID)
	}
}
