package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP request tracking
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "miraeboy_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "miraeboy_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})

	// Package operations
	ConanDownloadsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "miraeboy_conan_downloads_total",
		Help: "Total Conan package file downloads",
	}, []string{"repository"})

	ConanUploadsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "miraeboy_conan_uploads_total",
		Help: "Total Conan package file uploads",
	}, []string{"repository"})

	CargoDownloadsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "miraeboy_cargo_downloads_total",
		Help: "Total Cargo crate downloads",
	}, []string{"repository"})

	CargoPublishesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "miraeboy_cargo_publishes_total",
		Help: "Total Cargo crate publishes",
	}, []string{"repository"})

	// Repository/User gauges (updated periodically)
	RepositoriesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "miraeboy_repositories_total",
		Help: "Total number of repositories",
	})

	UsersTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "miraeboy_users_total",
		Help: "Total number of registered users",
	})
)
