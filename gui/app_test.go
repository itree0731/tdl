package main

import (
	"reflect"
	"testing"
)

func TestBuildUploadArgsPreservesPathsAndCoverOptions(t *testing.T) {
	req := UploadRequest{Namespace: "work", Chat: "123", Topic: 7, Paths: []string{`C:\media\one, two.mp4`, `D:\three.mp4`}, CoverMode: "video-cover", CoverAt: "12s", Threads: 8, Limit: 2}
	got := buildUploadArgs(req)
	want := []string{"--ns", "work", "--threads", "8", "--limit", "2", "up", "-p", `C:\media\one, two.mp4`, "-p", `D:\three.mp4`, "--chat", "123", "--topic", "7", "--cover-mode", "video-cover", "--cover-at", "12s"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args=%v\nwant=%v", got, want)
	}
}
