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

func TestBuildDownloadArgsUsesNonInteractiveResumeAndPreservesURLs(t *testing.T) {
	req := DownloadRequest{
		Namespace: "work",
		URLs:      []string{"https://t.me/c/123/4?single", "https://t.me/example/8"},
		Files:     []string{`C:\exports\one, two.json`},
		Directory: `D:\download target`,
		Threads:   8,
		Limit:     3,
		SkipSame:  true,
		Group:     true,
	}
	got := buildDownloadArgs(req)
	want := []string{"--ns", "work", "--threads", "8", "--limit", "3", "dl", "--dir", `D:\download target`, "--url", "https://t.me/c/123/4?single", "--url", "https://t.me/example/8", "--file", `C:\exports\one, two.json`, "--continue", "--skip-same", "--group"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args=%v\nwant=%v", got, want)
	}
}
