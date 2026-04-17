package domain

type TimeUnit int64

const (
	Second TimeUnit = 1
	Minute TimeUnit = 60
	Hour   TimeUnit = 3600
	Day    TimeUnit = 86400
	Month  TimeUnit = 2592000
)
