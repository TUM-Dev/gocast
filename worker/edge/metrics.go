package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
)
