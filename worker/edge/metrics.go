package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sync"
	"time"
)

var (
	playlistsRequested = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edge_playlists_requested_total",
		Help: "The total number of requested playlists",
	}, []string{"stream", "course"})

	chunksRequested = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edge_chunks_requested_total",
		Help: "The total number of ts chunks requested",
	}, []string{"stream", "course"})

	concurrentUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_concurrent_users",
		Help: "The number of concurrent users (users active in the last 5 minutes)",
	})

	usersMap = NewTTLMap(300)
)

// ttlmap for concurrent users:

type item struct {
	value      bool
	lastAccess int64
}

type TTLMap struct {
	m map[string]*item
	l sync.Mutex
}

func NewTTLMap(maxTTL int) (m *TTLMap) {
	m = &TTLMap{m: make(map[string]*item)}
	go func() {
		for now := range time.Tick(time.Second) {
			m.l.Lock()
			for k, v := range m.m {
				if now.Unix()-v.lastAccess > int64(maxTTL) {
					delete(m.m, k)
				}
			}
			m.l.Unlock()
		}
	}()
	return
}

func (m *TTLMap) Len() int {
	return len(m.m)
}

func (m *TTLMap) Put(k string, v bool) {
	m.l.Lock()
	it, ok := m.m[k]
	if !ok {
		it = &item{value: v}
		m.m[k] = it
	}
	it.lastAccess = time.Now().Unix()
	m.l.Unlock()
}

func (m *TTLMap) Get(k string) (v bool) {
	m.l.Lock()
	if it, ok := m.m[k]; ok {
		v = it.value
		it.lastAccess = time.Now().Unix()
	}
	m.l.Unlock()
	return
}
