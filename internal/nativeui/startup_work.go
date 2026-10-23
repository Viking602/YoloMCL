package nativeui

import "context"

// Background I/O is not on the UI thread, but decoding jars and thumbnails
// still competes for CPU, disk and GC during the first frame. Native windows
// release these workers after presentation. Headless renders/tests have no
// window lifecycle to wait for, so their nil gate runs immediately.
func waitForFirstFrame(ctx context.Context, ready <-chan struct{}) bool {
	if ready != nil {
		select {
		case <-ctx.Done():
			return false
		case <-ready:
		}
	}
	return ctx.Err() == nil
}
