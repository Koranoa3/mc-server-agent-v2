package container

// WorkingStatus はコンテナの稼働状況
type WorkingStatus int

const (
	StatusUnknown  WorkingStatus = iota
	StatusRunning                // 稼働中
	StatusStarting               // 起動中（ヘルスチェック待機）
	StatusStopped                // 停止中
	StatusNotFound               // 存在しない
)

// String は WorkingStatus を文字列に変換
func (s WorkingStatus) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusStarting:
		return "starting"
	case StatusStopped:
		return "stopped"
	case StatusNotFound:
		return "not_found"
	default:
		return "unknown"
	}
}

// JapaneseString は WorkingStatus を日本語に変換
func (s WorkingStatus) JapaneseString() string {
	switch s {
	case StatusRunning:
		return "稼働中"
	case StatusStarting:
		return "起動中"
	case StatusStopped:
		return "停止中"
	case StatusNotFound:
		return "存在しない"
	default:
		return "不明"
	}
}
