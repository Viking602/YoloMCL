// Package nativeui presents the launcher through MyGo's native widget tree.
// View state belongs to the UI thread; workers return results through App.post.
// Instance selection reuses cached data and never initiates a new system Java scan.
package nativeui
