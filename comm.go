package wserver

type Comm struct {
	request  *string
	response *string
	waitCH   chan struct{}
}
