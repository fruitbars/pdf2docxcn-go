package pdfconv

// UploadInfo contains information returned by RequestUpload.
type UploadInfo struct {
	TaskID        string            // 任务唯一标识
	UploadURL     string            // 文件上传地址
	UploadHeaders map[string]string // 上传时必须携带的 Headers
	CommitURL     string            // 便捷字段：commit 的完整路径（含 query）
	ExpireAt      int64             // 上传链接过期时间（Unix 时间戳，秒）
}

// Status represents the current state of a conversion task.
type Status struct {
	TaskID   string // 任务 ID
	Status   string // queued/processing/completed/failed
	Progress int    // 进度百分比 0-100
}

// UsageQuota holds total/used/remaining for a single quota dimension.
type UsageQuota struct {
	Total     int
	Used      int
	Remaining int
}

// Usage contains the current API quota status.
type Usage struct {
	Enabled     bool
	Description string
	CountMode   string // calls / pages / both
	Calls       UsageQuota
	Pages       UsageQuota
}

// DownloadInfo contains download information for a completed conversion.
type DownloadInfo struct {
	TaskID      string // 任务 ID
	DownloadURL string // 文件下载地址
	Filename    string // 建议的保存文件名
	ExpireAt    int64  // 下载链接建议有效期（Unix 时间戳，秒）
}
