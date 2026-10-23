package startup

import (
	"context"
	"reflect"
	"testing"
)

func TestDriverPreparationWaitsForPresentation(t *testing.T) {
	a := New()
	called := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.prepare(ctx, func(context.Context) { called <- struct{}{} })
	conn := testConnection{Event: func(testEvent) bool { return true }, Post: func(func()) {}}
	if !install(reflect.ValueOf(&conn).Elem(), a.done, func() { a.presentOnce.Do(func() { close(a.presented) }) }) {
		t.Fatal("connection rejected")
	}
	conn.Event(testEvent{Kind: surfaceFrame})
	select {
	case <-a.presented:
		t.Fatal("warming began before presentation")
	default:
	}
	conn.Ready()
	conn.Ready() // window lifecycle duplication cannot restart the worker
	<-a.done
	if len(called) != 1 {
		t.Fatal("preparation was not released exactly once")
	}
	conn.Detach()
}

func TestQuitBeforePresentationCancelsPreparation(t *testing.T) {
	a := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.prepare(ctx, func(context.Context) { t.Fatal("driver touched after quit") })
	select {
	case <-a.done:
	default:
		t.Fatal("cancelled preparation did not finish")
	}
}
