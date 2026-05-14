package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrokerPublishDeliveredToSubscriber(t *testing.T) {
	b := newBroker()
	_, ch := b.subscribe()
	b.publish("app.log", `{"message":"hello"}`)
	msg := <-ch
	assert.Equal(t, "app.log", msg.S)
	assert.Equal(t, `{"message":"hello"}`, msg.D)
}

func TestBrokerSubscribeReturnsHistory(t *testing.T) {
	b := newBroker()
	b.publish("a.log", "line1")
	b.publish("b.log", "line2")
	hist, _ := b.subscribe()
	assert.Len(t, hist, 2)
	assert.Equal(t, "line1", hist[0].D)
	assert.Equal(t, "line2", hist[1].D)
}

func TestBrokerPublishBatchDeliveredInOrder(t *testing.T) {
	b := newBroker()
	_, ch := b.subscribe()
	msgs := []logMsg{
		{S: "a.log", D: "line1"},
		{S: "b.log", D: "line2"},
		{S: "a.log", D: "line3"},
	}
	b.publishBatch(msgs)
	assert.Equal(t, logMsg{S: "a.log", D: "line1"}, <-ch)
	assert.Equal(t, logMsg{S: "b.log", D: "line2"}, <-ch)
	assert.Equal(t, logMsg{S: "a.log", D: "line3"}, <-ch)
}

func TestBrokerPublishBatchAppearsInHistory(t *testing.T) {
	b := newBroker()
	b.publishBatch([]logMsg{{S: "a.log", D: "line1"}, {S: "b.log", D: "line2"}})
	hist, _ := b.subscribe()
	assert.Len(t, hist, 2)
}

func TestBrokerHistoryBoundedAtMaxHistory(t *testing.T) {
	b := newBroker()
	for range maxHistory + 10 {
		b.publish("x.log", "line")
	}
	hist, _ := b.subscribe()
	assert.Len(t, hist, maxHistory)
}

func TestBrokerUnsubscribeStopsDelivery(t *testing.T) {
	b := newBroker()
	_, ch := b.subscribe()
	b.unsubscribe(ch)
	b.publish("a.log", "line")
	assert.Zero(t, len(ch))
}
