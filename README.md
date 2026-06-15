# pdf2docx.cn Go SDK 开发文档

Go SDK for `pdf2docx.cn` OpenAPI v2 — 一个对接文件转换服务的 Go 客户端库。

---

## 目录

- [快速开始](#快速开始)
- [安装](#安装)
- [核心概念](#核心概念)
- [API 参考](#api-参考)
  - [创建客户端](#创建客户端)
  - [Convert（便捷方法）](#convert便捷方法)
  - [RequestUpload](#requestupload)
  - [UploadFile](#uploadfile)
  - [CommitConversion](#commitconversion)
  - [GetStatus](#getstatus)
  - [GetDownloadURL](#getdownloadurl)
  - [DownloadFile](#downloadfile)
- [错误处理](#错误处理)
- [并发使用](#并发使用)
- [Context 控制](#context-控制)
- [完整示例](#完整示例)
- [支持的转换类型](#支持的转换类型)
- [GetUsage](#getusage)
- [限制与配额](#限制与配额)
- [常见问题](#常见问题)

---

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/fruitbars/pdf2docxcn-go/pdfconv"
)

func main() {
    client := pdfconv.NewClient(
        "https://api.pdf2docx.cn",
        "YOUR_API_KEY",
        "YOUR_API_SECRET",
    )

    output, err := client.Convert(
        context.Background(),
        "input.pdf",  // 输入文件
        "./output",   // 输出目录
        "pdf2word",   // 转换类型
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("转换完成: %s\n", output)
}
```

就这么简单。一行代码完成上传、转换、下载全流程。

---

## 安装

```bash
go get github.com/fruitbars/pdf2docxcn-go
```

要求 Go 1.19+，无第三方依赖（仅使用标准库）。

---

## 核心概念

### 转换流程的 6 个步骤

一次完整的文件转换在内部分为 6 步：

```
① 申请上传     RequestUpload      → 获得 taskID + 上传 URL
② 上传文件     UploadFile         → PUT 文件到 R2
③ 提交转换     CommitConversion   → 告诉服务器开始转换
④ 查询状态     GetStatus          → 轮询直到 completed
⑤ 获取下载链接  GetDownloadURL    → 获得最终文件 URL
⑥ 下载文件     DownloadFile       → 保存到本地
```

SDK 提供两种使用方式：

- **便捷模式**：调用 `Convert()`，SDK 自动跑完 6 步
- **细粒度模式**：开发者自己调用 6 个独立方法，可以自定义轮询策略、错误处理、进度反馈等

### 认证机制

每次 API 请求都需要 4 个 HMAC-SHA256 签名 Header（`X-Api-Key`、`X-Timestamp`、`X-Nonce`、`X-Signature`）。**SDK 内部自动处理签名**，开发者只需提供 `apiKey` 和 `apiSecret`。

文件上传（步骤 ②）和文件下载（步骤 ⑥）走预签名的 URL，**不需要 API 签名**——SDK 也已经处理好了。

---

## API 参考

### 创建客户端

#### `NewClient(host, apiKey, apiSecret string) *Client`

使用默认 HTTP 配置创建客户端（120 秒超时）。

```go
client := pdfconv.NewClient(
    "https://api.pdf2docx.cn",
    "YOUR_API_KEY",
    "YOUR_API_SECRET",
)
```

#### `NewClientWithHTTP(host, apiKey, apiSecret string, httpClient *http.Client) *Client`

自定义 HTTP 客户端，适用于需要代理、自定义 TLS、连接池等场景。

```go
httpClient := &http.Client{
    Timeout: 5 * time.Minute,
    Transport: &http.Transport{
        MaxIdleConnsPerHost: 20,
        DisableKeepAlives:   false,
    },
}
client := pdfconv.NewClientWithHTTP(host, apiKey, apiSecret, httpClient)
```

> `Client` 是**并发安全**的——多个 goroutine 可以共享同一个实例。建议整个应用复用一个 Client。

---

### Convert（便捷方法）

```go
func (c *Client) Convert(ctx context.Context, inputPath, outputDir, convType string) (string, error)
```

一站式转换：内部按顺序调用 6 个方法，**阻塞直到转换完成**。

**参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| `ctx` | `context.Context` | 上下文，支持超时和取消 |
| `inputPath` | `string` | 输入文件本地路径 |
| `outputDir` | `string` | 输出目录（如果不存在会自动创建） |
| `convType` | `string` | 转换类型，如 `"pdf2word"` |

**返回：**
- `string`：转换后文件的完整路径（如 `./output/report.docx`）
- `error`：任何一步失败都会返回错误

**示例：**

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

outputPath, err := client.Convert(ctx, "report.pdf", "./output", "pdf2word")
if err != nil {
    log.Fatalf("转换失败: %v", err)
}
log.Printf("输出文件: %s", outputPath)
```

> 轮询间隔固定为 5 秒。如果需要不同的策略（比如先快后慢、按文件大小调整），用下面的细粒度 API。

---

### RequestUpload

```go
func (c *Client) RequestUpload(ctx context.Context, filename, convType string, size int64) (*UploadInfo, error)
```

申请上传，获得 `taskID` 和预签名的上传 URL。

**参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| `filename` | `string` | 原始文件名（用于命名转换结果，如 `report.pdf` → `report.docx`） |
| `convType` | `string` | 转换类型 |
| `size` | `int64` | 文件字节数（必须与实际上传大小一致，误差超过 10% 会被拒绝） |

**返回 `*UploadInfo`：**

```go
type UploadInfo struct {
    TaskID        string            // 任务唯一标识，后续接口都要用
    UploadURL     string            // 预签名上传 URL
    UploadHeaders map[string]string // 上传时必须原样带上的 Headers
    CommitURL     string            // 便捷字段：commit 的完整路径
    ExpireAt      int64             // 上传链接过期时间（Unix 秒）
}
```

**示例：**

```go
fileInfo, _ := os.Stat("report.pdf")
upload, err := client.RequestUpload(ctx, "report.pdf", "pdf2word", fileInfo.Size())
if err != nil {
    return err
}
fmt.Printf("任务 ID: %s\n", upload.TaskID)
fmt.Printf("上传链接 %d 秒后过期\n", upload.ExpireAt-time.Now().Unix())
```

> ⚠️ 上传 URL **15 分钟内有效，且只能使用一次**。过期或失败后需要重新调用 `RequestUpload`。

---

### UploadFile

```go
func (c *Client) UploadFile(ctx context.Context, uploadURL string, uploadHeaders map[string]string, filePath string) error
```

把本地文件 PUT 到预签名 URL。**此步骤不走 API 签名**，认证已经在 `uploadHeaders` 里。

**参数：**
| 参数 | 类型 | 说明 |
|---|---|---|
| `uploadURL` | `string` | `RequestUpload` 返回的 `UploadURL` |
| `uploadHeaders` | `map[string]string` | `RequestUpload` 返回的 `UploadHeaders`（必须原样带上） |
| `filePath` | `string` | 本地文件路径 |

**示例：**

```go
err := client.UploadFile(ctx, upload.UploadURL, upload.UploadHeaders, "report.pdf")
if err != nil {
    return err
}
```

> ⚠️ **不要修改 `uploadHeaders` 里的任何字段**，包括不要追加自己的 Header。服务器会校验所有签名相关的 Header。

---

### CommitConversion

```go
func (c *Client) CommitConversion(ctx context.Context, taskID string) error
```

通知服务器文件已上传完毕，启动异步转换。

**示例：**

```go
if err := client.CommitConversion(ctx, upload.TaskID); err != nil {
    return err
}
fmt.Println("转换已排队")
```

**常见错误：**
| 错误 code | HTTP | 含义 |
|---|---|---|
| `source_not_uploaded` | 409 | 文件未上传完成或 token 未消费 |
| `size_mismatch` | 400 | 实际大小与申请时声明的差异超过 10% |
| `size_too_large` | 413 | 文件超过 50MB |

---

### GetStatus

```go
func (c *Client) GetStatus(ctx context.Context, taskID string) (*Status, error)
```

查询任务状态。

**返回 `*Status`：**

```go
type Status struct {
    TaskID   string // 任务 ID
    Status   string // queued / processing / completed / failed
    Progress int    // 进度百分比 0-100（仅 processing 状态下有意义）
}
```

**状态值：**

| `Status` | 含义 | 下一步 |
|---|---|---|
| `queued` | 已排队 | 继续轮询 |
| `processing` | 转换中 | 继续轮询 |
| `completed` | 已完成 | 调用 `GetDownloadURL` |
| `failed` | 失败 | 终止流程 |

**示例：自定义轮询策略（先快后慢）：**

```go
intervals := []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second, 5 * time.Second}
idx := 0

for {
    if idx < len(intervals) {
        time.Sleep(intervals[idx])
        idx++
    } else {
        time.Sleep(5 * time.Second)
    }

    status, err := client.GetStatus(ctx, taskID)
    if err != nil {
        return err
    }

    fmt.Printf("状态: %s, 进度: %d%%\n", status.Status, status.Progress)

    if status.Status == "completed" {
        break
    }
    if status.Status == "failed" {
        return fmt.Errorf("转换失败")
    }
}
```

> 服务端建议轮询间隔不低于 **5 秒**，过于频繁可能触发限流。如果你的场景对延迟敏感（小文件），可以适当缩短到 2-3 秒。

---

### GetDownloadURL

```go
func (c *Client) GetDownloadURL(ctx context.Context, taskID string) (*DownloadInfo, error)
```

获取转换结果的下载链接。**前置条件**：`status` 必须是 `completed`，否则返回 409 错误。

**返回 `*DownloadInfo`：**

```go
type DownloadInfo struct {
    TaskID      string // 任务 ID
    DownloadURL string // 公开下载 URL（无需认证）
    Filename    string // 建议的保存文件名（已带正确扩展名）
    ExpireAt    int64  // 下载链接建议有效期（Unix 秒）
}
```

**示例：**

```go
dl, err := client.GetDownloadURL(ctx, taskID)
if err != nil {
    return err
}
fmt.Printf("下载地址: %s\n", dl.DownloadURL)
fmt.Printf("建议文件名: %s\n", dl.Filename)
```

---

### DownloadFile

```go
func (c *Client) DownloadFile(ctx context.Context, downloadURL, outputPath string) error
```

下载文件到本地路径。**此步骤不需要任何认证 Header**——URL 已经预签名。

**参数：**
| 参数 | 类型 | 说明 |
|---|---|---|
| `downloadURL` | `string` | `GetDownloadURL` 返回的 URL |
| `outputPath` | `string` | 本地保存路径（父目录会自动创建） |

**示例：**

```go
outputPath := filepath.Join("./output", dl.Filename)
if err := client.DownloadFile(ctx, dl.DownloadURL, outputPath); err != nil {
    return err
}
```

---

## 错误处理

SDK **不做自动重试**——所有错误直接返回，由调用方决定是否重试。

### 错误类型

#### `*APIError` — 来自服务端的业务错误

```go
type APIError struct {
    Code       string // 错误码，如 "unauthorized"、"quota_exhausted"
    Message    string // 人类可读的描述
    HTTPStatus int    // HTTP 状态码
}
```

判断方式：

```go
import "errors"

_, err := client.RequestUpload(ctx, "f.pdf", "pdf2word", 1024)
if err != nil {
    var apiErr *pdfconv.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.Code {
        case "unauthorized":
            log.Fatal("API Key 错误或时间偏差过大")
        case "quota_exhausted":
            log.Fatal("配额用尽")
        case "size_too_large":
            log.Fatal("文件超过 50MB")
        default:
            log.Fatalf("API 错误: %s", apiErr.Message)
        }
    } else {
        // 网络错误、超时、文件读写错误等
        log.Fatalf("非 API 错误: %v", err)
    }
}
```

#### 常见 API 错误码

| `Code` | HTTP | 说明 |
|---|---|---|
| `unauthorized` | 401 | 签名失败（密钥错、时间偏差超过 ±5 分钟、nonce 重复） |
| `quota_exhausted` | 402 | 配额不足 |
| `invalid_request` | 400 | 参数错误 |
| `invalid_type` | 400 | 不支持的 `convType` |
| `size_too_large` | 413 | 文件超过 50MB |
| `size_mismatch` | 400 | 实际大小与声明不符 |
| `source_not_uploaded` | 409 | commit 时文件未上传完成 |
| `not_ready` | 409 | 下载时转换尚未完成 |
| `missing_token` | 401 | 上传时缺少 `X-Upload-Token` |
| `token_not_found` | 401 | 上传 token 过期或已使用 |
| `service_unavailable` | 503 | 服务暂时不可用，稍后重试 |

### 重试建议

| 错误类型 | 是否应该重试 |
|---|---|
| 网络超时 / `context.DeadlineExceeded` | ✅ 可以，建议指数退避 |
| `503 service_unavailable` | ✅ 可以，等几秒再试 |
| `401 unauthorized` | ❌ 不要，检查密钥和时钟 |
| `402 quota_exhausted` | ❌ 不要，等配额恢复 |
| `400 invalid_*` | ❌ 不要，修代码 |
| `409 source_not_uploaded` | ⚠️ 需要重新走完整流程（重新申请上传） |
| `409 not_ready` | ✅ 继续轮询 status |

---

## 并发使用

`Client` 是**并发安全**的——内部所有字段创建后只读，每次请求生成独立的 nonce 和 timestamp。

### 推荐模式：单 Client + 多 goroutine

```go
// 全局只创建一次
var client = pdfconv.NewClient(host, apiKey, apiSecret)

func batchConvert(files []string) {
    var wg sync.WaitGroup
    sem := make(chan struct{}, 5)  // 限制并发数为 5

    for _, file := range files {
        wg.Add(1)
        go func(f string) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()

            output, err := client.Convert(context.Background(), f, "./output", "pdf2word")
            if err != nil {
                log.Printf("[%s] 失败: %v", f, err)
                return
            }
            log.Printf("[%s] → %s", f, output)
        }(file)
    }
    wg.Wait()
}
```

### 并发的几个注意点

1. **每次请求自动用独立的 nonce/timestamp** — 不会有签名冲突
2. **底层 `http.Client` 有连接池** — 多 goroutine 共享高效
3. **`Context` 独立传递** — 每个 goroutine 可以有自己的超时和取消
4. **配额是共享的** — 并发不会绕过配额限制
5. **服务端限流按 API Key 计算** — 大量并发可能触发限流，建议自己控制并发数（推荐 5-10）

---

## Context 控制

### 设置整体超时

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

output, err := client.Convert(ctx, "big.pdf", "./output", "pdf2word")
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("转换超时")
}
```

### 中途取消

```go
ctx, cancel := context.WithCancel(context.Background())

// 在另一个 goroutine 里监听信号
go func() {
    <-someSignal
    cancel()  // 取消转换
}()

output, err := client.Convert(ctx, "file.pdf", "./output", "pdf2word")
```

### 单个方法超时

如果只想给某一步设置超时（比如下载），把 `context` 传给那个方法即可：

```go
// 全流程用 30 分钟
ctx30 := context.Background()

upload, _ := client.RequestUpload(ctx30, ...)
_ = client.UploadFile(ctx30, ...)
_ = client.CommitConversion(ctx30, upload.TaskID)

// 但是下载这一步只给 1 分钟
ctxDl, cancel := context.WithTimeout(ctx30, 1*time.Minute)
defer cancel()
_ = client.DownloadFile(ctxDl, dlInfo.DownloadURL, "out.docx")
```

---

## 完整示例

### 示例 1：最简单 — 转一个文件

```go
package main

import (
    "context"
    "log"

    "github.com/fruitbars/pdf2docxcn-go/pdfconv"
)

func main() {
    client := pdfconv.NewClient("https://api.pdf2docx.cn", "KEY", "SECRET")
    output, err := client.Convert(context.Background(), "input.pdf", "./output", "pdf2word")
    if err != nil {
        log.Fatal(err)
    }
    log.Println("输出:", output)
}
```

### 示例 2：并发批量转换

```go
package main

import (
    "context"
    "log"
    "sync"

    "github.com/fruitbars/pdf2docxcn-go/pdfconv"
)

func main() {
    client := pdfconv.NewClient("https://api.pdf2docx.cn", "KEY", "SECRET")

    files := []string{"a.pdf", "b.pdf", "c.pdf", "d.pdf", "e.pdf"}
    sem := make(chan struct{}, 3)  // 最多 3 个并发
    var wg sync.WaitGroup

    for _, f := range files {
        wg.Add(1)
        go func(file string) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()

            output, err := client.Convert(context.Background(), file, "./output", "pdf2word")
            if err != nil {
                log.Printf("[%s] 失败: %v", file, err)
                return
            }
            log.Printf("[%s] → %s", file, output)
        }(f)
    }
    wg.Wait()
}
```

### 示例 3：细粒度控制 + 自定义轮询 + 进度回调

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "time"

    "github.com/fruitbars/pdf2docxcn-go/pdfconv"
)

type ProgressCallback func(stage string, progress int)

func ConvertWithProgress(client *pdfconv.Client, inputPath, outputDir, convType string, cb ProgressCallback) (string, error) {
    ctx := context.Background()

    // 1. 申请上传
    cb("upload_request", 0)
    stat, err := os.Stat(inputPath)
    if err != nil {
        return "", err
    }
    upload, err := client.RequestUpload(ctx, filepath.Base(inputPath), convType, stat.Size())
    if err != nil {
        return "", fmt.Errorf("申请上传: %w", err)
    }

    // 2. 上传文件
    cb("uploading", 10)
    if err := client.UploadFile(ctx, upload.UploadURL, upload.UploadHeaders, inputPath); err != nil {
        return "", fmt.Errorf("上传: %w", err)
    }

    // 3. 提交转换
    cb("committing", 30)
    if err := client.CommitConversion(ctx, upload.TaskID); err != nil {
        return "", fmt.Errorf("提交: %w", err)
    }

    // 4. 自定义轮询（先快后慢）
    intervals := []time.Duration{2 * time.Second, 3 * time.Second, 5 * time.Second}
    idx := 0
    for {
        if idx < len(intervals) {
            time.Sleep(intervals[idx])
            idx++
        } else {
            time.Sleep(5 * time.Second)
        }

        status, err := client.GetStatus(ctx, upload.TaskID)
        if err != nil {
            return "", fmt.Errorf("查询状态: %w", err)
        }

        cb("processing", 30+status.Progress*60/100)

        if status.Status == "completed" {
            break
        }
        if status.Status == "failed" {
            return "", fmt.Errorf("转换失败")
        }
    }

    // 5. 获取下载链接
    cb("download_url", 90)
    dl, err := client.GetDownloadURL(ctx, upload.TaskID)
    if err != nil {
        return "", fmt.Errorf("获取下载链接: %w", err)
    }

    // 6. 下载文件
    cb("downloading", 95)
    output := filepath.Join(outputDir, dl.Filename)
    if err := client.DownloadFile(ctx, dl.DownloadURL, output); err != nil {
        return "", fmt.Errorf("下载: %w", err)
    }

    cb("done", 100)
    return output, nil
}

func main() {
    client := pdfconv.NewClient("https://api.pdf2docx.cn", "KEY", "SECRET")

    output, err := ConvertWithProgress(client, "input.pdf", "./output", "pdf2word",
        func(stage string, progress int) {
            log.Printf("[%d%%] %s", progress, stage)
        })
    if err != nil {
        log.Fatal(err)
    }
    log.Println("输出:", output)
}
```

### 示例 4：带超时和错误重试

```go
package main

import (
    "context"
    "errors"
    "log"
    "time"

    "github.com/fruitbars/pdf2docxcn-go/pdfconv"
)

func convertWithRetry(client *pdfconv.Client, inputPath string) (string, error) {
    const maxAttempts = 3

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
        output, err := client.Convert(ctx, inputPath, "./output", "pdf2word")
        cancel()

        if err == nil {
            return output, nil
        }

        // 判断是否值得重试
        var apiErr *pdfconv.APIError
        if errors.As(err, &apiErr) {
            switch apiErr.Code {
            case "unauthorized", "quota_exhausted", "invalid_request", "invalid_type", "size_too_large":
                // 这些错误重试没用
                return "", err
            }
        }

        log.Printf("第 %d 次失败: %v，5 秒后重试", attempt, err)
        time.Sleep(5 * time.Second)
    }

    return "", errors.New("重试 3 次仍失败")
}
```

---

## 支持的转换类型

| `convType` | 输入 → 输出 |
|---|---|
| `pdf2word` | PDF → .docx |
| `pdf2excel` | PDF → .xlsx |
| `pdf2ppt` | PDF → .pptx |
| `pdf2txt` | PDF → .txt |
| `pdf2md` | PDF → .md |
| `word2pdf` | Word → .pdf |
| `excel2pdf` | Excel → .pdf |
| `ppt2pdf` | PowerPoint → .pdf |
| `ofd2pdf` | OFD → .pdf |
| `word2md` | Word → .md |
| `excel2md` | Excel → .md |
| `word2txt` | Word → .txt |

---

### GetUsage

```go
func (c *Client) GetUsage(ctx context.Context) (*Usage, error)
```

查询当前 API Key 的配额使用情况。

**返回 `*Usage`：**

```go
type UsageQuota struct {
    Total     int
    Used      int
    Remaining int // 可能为负数（超额使用时）
}

type Usage struct {
    Enabled     bool
    Description string
    CountMode   string // "calls" / "pages" / "both"
    Calls       UsageQuota
    Pages       UsageQuota
}
```

**示例：**

```go
usage, err := client.GetUsage(context.Background())
if err != nil {
    log.Fatal(err)
}
fmt.Printf("计费模式: %s\n", usage.CountMode)
fmt.Printf("调用次数: 已用 %d / 总计 %d，剩余 %d\n",
    usage.Calls.Used, usage.Calls.Total, usage.Calls.Remaining)
fmt.Printf("页数: 已用 %d / 总计 %d，剩余 %d\n",
    usage.Pages.Used, usage.Pages.Total, usage.Pages.Remaining)
```

**计费模式说明：**

| `CountMode` | 含义 |
|---|---|
| `calls` | 按调用次数计费，`Calls.Remaining` 耗尽即停止 |
| `pages` | 按页数计费，`Pages.Remaining` 耗尽即停止 |
| `both` | 两者都计，任意一个耗尽即停止（默认模式） |

> 建议在调用转换前先检查配额，避免在转换成功后才发现配额不足。

---

## 限制与配额

| 项目 | 限制 |
|---|---|
| 单文件大小 | 50 MB |
| 上传链接有效期 | 15 分钟（一次性） |
| 下载链接有效期 | 30 分钟 |
| 转换超时 | 15 分钟 |
| 签名时间窗口 | ±5 分钟（GET）/ ±15 分钟（POST/PUT） |
| Nonce 防重放 | 20 分钟内不可重复 |

**配额扣减规则：**
- 转换**成功**才扣配额
- 失败不扣
- 三种计费模式：按次（`calls`）、按页（`pages`）、两者取先到（`both`）

---

## 常见问题

**Q: SDK 会自动重试吗？**

不会。所有错误直接返回。这样设计是因为不同的错误类型适合不同的重试策略，SDK 不替你做决定。重试逻辑参考 [示例 4](#示例-4带超时和错误重试)。

**Q: 我应该用 `Convert` 还是细粒度 API？**

- 不在意进度、不需要自定义轮询、不并发上传/下载 → 用 `Convert`
- 需要进度反馈给前端 / 自定义轮询间隔 / 分阶段处理 → 用 6 个细粒度方法

**Q: `Client` 可以多 goroutine 共享吗？**

可以。`Client` 内部只读，并发安全。建议整个应用复用一个 `Client` 实例（HTTP 连接池效率更高）。

**Q: 上传失败后如何重试？**

上传 URL 是**一次性**的。如果 `UploadFile` 失败（或上传过程网络中断），需要**从 `RequestUpload` 重新开始**——会拿到新的 `taskID` 和新的上传 URL。

**Q: 怎么取消正在进行的转换？**

用 `context` 取消。但注意：**取消只能停止 SDK 这一端的等待**——服务端的转换任务无法主动停止（会自然超时清理）。

```go
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(10 * time.Second)
    cancel()  // 10 秒后中断
}()
_, err := client.Convert(ctx, ...)
// err 是 context.Canceled
```

**Q: 时间戳偏差导致 `unauthorized` 怎么办？**

SDK 用 `time.Now().Unix()` 拿时间戳。如果服务器和客户端时钟偏差超过 5 分钟（GET）或 15 分钟（POST/PUT），会被拒绝。解决方法：

1. 同步系统时间（NTP）
2. 如果是容器环境，确保容器时间正确

**Q: 文件能直接传 `io.Reader` 而不是路径吗？**

当前版本只接受文件路径。如果有需求可以提 Issue，未来版本会考虑增加 `UploadReader` 方法。

**Q: 上传大文件需要分块吗？**

不需要。50MB 以内整体 PUT 即可。R2 后端自己处理。

---

## 版本与兼容性

- **当前版本：** v1.0.0
- **API 版本：** OpenAPI v2
- **Go 版本要求：** Go 1.19+
- **第三方依赖：** 无（仅使用标准库）

---

## 反馈与支持

- Issue：https://github.com/fruitbars/pdf2docxcn-go/issues
- 服务文档：https://pdf2docx.cn

---

*文档版本 1.0 | 更新时间 2026-06-15*
