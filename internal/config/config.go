package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	MediaMTX MediaMTXConfig `mapstructure:"mediamtx"`
	FFmpeg   FFmpegConfig   `mapstructure:"ffmpeg"`
	Ytdlp    YtdlpConfig    `mapstructure:"ytdlp"`
	Monitor  MonitorConfig  `mapstructure:"monitor"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// ServerConfig holds RTSP server settings
type ServerConfig struct {
	RTSPPort int `mapstructure:"rtsp_port"`
	APIPort  int `mapstructure:"api_port"`
}

// MediaMTXConfig holds MediaMTX binary and config settings
type MediaMTXConfig struct {
	BinaryPath string `mapstructure:"binary_path"`
	ConfigPath string `mapstructure:"config_path"`
	LogLevel   string `mapstructure:"log_level"`
}

// FFmpegConfig holds FFmpeg settings
type FFmpegConfig struct {
	BinaryPath    string   `mapstructure:"binary_path"`
	InputOptions  []string `mapstructure:"input_options"`
	OutputOptions []string `mapstructure:"output_options"`
}

// YtdlpConfig holds yt-dlp settings
type YtdlpConfig struct {
	BinaryPath string        `mapstructure:"binary_path"`
	Timeout    time.Duration `mapstructure:"timeout"`
	Format     string        `mapstructure:"format"`
}

// MonitorConfig holds monitoring settings
type MonitorConfig struct {
	HealthCheckInterval  time.Duration   `mapstructure:"health_check_interval"`
	URLRefreshInterval   time.Duration   `mapstructure:"url_refresh_interval"`
	MaxConsecutiveErrors int             `mapstructure:"max_consecutive_errors"`
	Reconnect            ReconnectConfig `mapstructure:"reconnect"`
}

// ReconnectConfig holds reconnection settings
type ReconnectConfig struct {
	InitialDelay time.Duration `mapstructure:"initial_delay"`
	MaxDelay     time.Duration `mapstructure:"max_delay"`
	Multiplier   float64       `mapstructure:"multiplier"`
	MaxAttempts  int           `mapstructure:"max_attempts"`
}

// StorageConfig holds storage settings
type StorageConfig struct {
	DataDir string `mapstructure:"data_dir"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	File   string `mapstructure:"file"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Config file settings
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("/etc/youtube-rtsp-proxy")
		v.AddConfigPath("$HOME/.youtube-rtsp-proxy")
		v.AddConfigPath(".")
	}

	// Environment variables
	v.SetEnvPrefix("YTRTSP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		// Config file not found, use defaults
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Resolve paths
	cfg.resolveDataDir()

	return &cfg, nil
}

// setDefaults sets default values for configuration
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.rtsp_port", 8554)
	v.SetDefault("server.api_port", 9997)

	// MediaMTX defaults
	v.SetDefault("mediamtx.binary_path", "mediamtx")
	v.SetDefault("mediamtx.config_path", "")
	v.SetDefault("mediamtx.log_level", "info")

	// FFmpeg defaults
	v.SetDefault("ffmpeg.binary_path", "ffmpeg")
	v.SetDefault("ffmpeg.input_options", []string{
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
	})
	v.SetDefault("ffmpeg.output_options", []string{
		"-c:v", "copy",
		"-c:a", "aac",
		"-f", "rtsp",
	})

	// yt-dlp defaults
	v.SetDefault("ytdlp.binary_path", "yt-dlp")
	v.SetDefault("ytdlp.timeout", 30*time.Second)
	v.SetDefault("ytdlp.format", "best[protocol=https]/best")

	// Monitor defaults
	v.SetDefault("monitor.health_check_interval", 30*time.Second)
	v.SetDefault("monitor.url_refresh_interval", 30*time.Minute)
	v.SetDefault("monitor.max_consecutive_errors", 3)
	v.SetDefault("monitor.reconnect.initial_delay", 5*time.Second)
	v.SetDefault("monitor.reconnect.max_delay", 5*time.Minute)
	v.SetDefault("monitor.reconnect.multiplier", 2.0)
	v.SetDefault("monitor.reconnect.max_attempts", 10)

	// Storage defaults
	v.SetDefault("storage.data_dir", "")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "text")
	v.SetDefault("logging.file", "")
}

// resolveDataDir resolves the data directory path
func (c *Config) resolveDataDir() {
	if c.Storage.DataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			c.Storage.DataDir = "/tmp/youtube-rtsp-proxy"
		} else {
			c.Storage.DataDir = filepath.Join(homeDir, ".local", "share", "youtube-rtsp-proxy")
		}
	}
}

// GetMediaMTXConfigPath returns the MediaMTX config path, creating default if needed
func (c *Config) GetMediaMTXConfigPath() string {
	if c.MediaMTX.ConfigPath != "" {
		return c.MediaMTX.ConfigPath
	}
	return filepath.Join(c.Storage.DataDir, "mediamtx.yml")
}

// GetRTSPURL returns the full RTSP URL for a given path
func (c *Config) GetRTSPURL(path string) string {
	return "rtsp://localhost:" + strings.TrimPrefix(path, "/") + "/" + path
}
