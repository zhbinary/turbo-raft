//Created by zhbinary on 2019-06-03.
//Email: zhbinary@gmail.com
package utils

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

type Output int

const (
	FILE Output = iota
	CONSOLE
	ALL
)

type LoggerConfig struct {
	LogPath   string
	LogName   string
	LogLevel  Level
	MaxSize   int
	MaxBackup int
	MaxAge    int
	LogOutput Output
}

func NewLoggerInfo() *LoggerConfig {
	return &LoggerConfig{LogLevel: DEBUG, LogOutput: CONSOLE}
}
