package main

import (
	"reflect"
	"testing"
)

func TestBuildUploadArgsPreservesPathsAndCoverOptions(t *testing.T) {
	req := UploadRequest{Namespace: "work", Proxy: "socks5://127.0.0.1:1080", Chat: "123", Topic: 7, Paths: []string{`C:\media\one, two.mp4`, `D:\three.mp4`}, CoverMode: "video-cover", CoverAt: "12s", Threads: 8, Limit: 2}
	got := buildUploadArgs(req)
	want := []string{"--ns", "work", "--proxy", "socks5://127.0.0.1:1080", "--threads", "8", "--limit", "2", "up", "-p", `C:\media\one, two.mp4`, "-p", `D:\three.mp4`, "--chat", "123", "--topic", "7", "--cover-mode", "video-cover", "--cover-at", "12s"}
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

func TestBuildForwardArgsPreservesSourcesAndMode(t *testing.T) {
	req := ForwardRequest{Namespace: "work", Proxy: "socks5://127.0.0.1:1080", Threads: 6, From: []string{"https://t.me/c/1/2", `C:\exports\one, two.json`}, To: "12345", Mode: "clone", Silent: true}
	got := buildForwardArgs(req)
	want := []string{"--ns", "work", "--proxy", "socks5://127.0.0.1:1080", "--threads", "6", "forward", "--from", "https://t.me/c/1/2", "--from", `C:\exports\one, two.json`, "--to", "12345", "--mode", "clone", "--silent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args=%v\nwant=%v", got, want)
	}
}

func TestBuildChatExportArgsUsesLastRange(t *testing.T) {
	req := ChatExportRequest{Namespace: "default", Chat: "777", Topic: 9, Last: 50, Output: `D:\exports\chat.json`, WithContent: true}
	got := buildChatExportArgs(req)
	want := []string{"--ns", "default", "chat", "export", "--type", "last", "--input", "50", "--output", `D:\exports\chat.json`, "--chat", "777", "--topic", "9", "--with-content"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args=%v\nwant=%v", got, want)
	}
}
