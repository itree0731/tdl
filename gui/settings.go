package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/iyear/tdl/core/util/netutil"
	"github.com/iyear/tdl/pkg/consts"
)

const desktopSettingsVersion = 1

type DesktopSettings struct {
	SchemaVersion     int    `json:"schemaVersion"`
	Namespace         string `json:"namespace"`
	Proxy             string `json:"proxy"`
	Threads           int    `json:"threads"`
	Limit             int    `json:"limit"`
	DownloadDirectory string `json:"downloadDirectory"`
	CoverMode         string `json:"coverMode"`
	CoverAt           string `json:"coverAt"`
	DownloadGroup     bool   `json:"downloadGroup"`
	DownloadSkipSame  bool   `json:"downloadSkipSame"`
	DownloadRewrite   bool   `json:"downloadRewrite"`
}

func defaultDesktopSettings() DesktopSettings {
	return DesktopSettings{
		SchemaVersion:     desktopSettingsVersion,
		Namespace:         "default",
		Threads:           4,
		Limit:             2,
		DownloadDirectory: "downloads",
		CoverMode:         "video-cover",
		CoverAt:           "auto",
		DownloadGroup:     true,
		DownloadSkipSame:  true,
	}
}

func desktopSettingsPath() string { return filepath.Join(consts.DataDir, "tmt.json") }

func loadDesktopSettings(path string) (DesktopSettings, error) {
	settings := defaultDesktopSettings()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if err = json.Unmarshal(data, &settings); err != nil {
		return defaultDesktopSettings(), fmt.Errorf("读取 GUI 设置: %w", err)
	}
	settings.SchemaVersion = desktopSettingsVersion
	if err = validateDesktopSettings(settings); err != nil {
		return defaultDesktopSettings(), fmt.Errorf("GUI 设置无效: %w", err)
	}
	return settings, nil
}

func validateDesktopSettings(settings DesktopSettings) error {
	if settings.Threads < 1 || settings.Threads > 64 {
		return fmt.Errorf("单文件线程数必须在 1 到 64 之间")
	}
	if settings.Limit < 1 || settings.Limit > 32 {
		return fmt.Errorf("并行任务数必须在 1 到 32 之间")
	}
	if settings.Namespace == "" {
		return fmt.Errorf("默认账号不能为空")
	}
	if settings.DownloadDirectory == "" {
		return fmt.Errorf("默认下载目录不能为空")
	}
	if settings.CoverMode != "video-cover" && settings.CoverMode != "thumbnail" && settings.CoverMode != "off" {
		return fmt.Errorf("视频封面模式无效")
	}
	if settings.CoverAt == "" {
		return fmt.Errorf("封面时间不能为空")
	}
	if settings.Proxy != "" {
		parsed, err := url.Parse(settings.Proxy)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("代理地址格式无效")
		}
		if _, err = netutil.NewProxy(settings.Proxy); err != nil {
			return fmt.Errorf("代理地址无效: %w", err)
		}
	}
	return nil
}

func saveDesktopSettings(path string, settings DesktopSettings) error {
	settings.SchemaVersion = desktopSettingsVersion
	if err := validateDesktopSettings(settings); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".gui-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err = temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
