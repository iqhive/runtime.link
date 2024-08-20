package cpu

type TimeToLive interface {
	Done() <-chan struct{}
}
