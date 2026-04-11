package redis

import "errors"

var Nil = errors.New("redis: nil")

type StringCmd struct {
	err error
}

func (c *StringCmd) Err() error {
	return c.err
}

type Client struct{}

func (c *Client) Get(_ any, _ string) *StringCmd {
	return &StringCmd{err: Nil}
}
