package provider

import (
	"context"
	"time"
)

// GistFile 单个 gist 文件的元数据与内容。json tag 与 GitHub /gists 响应对齐,
// 与消费方(git-sync-service githubapi.Gist 内嵌结构)字段保持兼容。
type GistFile struct {
	Filename string `json:"filename"`
	Language string `json:"language"`
	RawURL   string `json:"raw_url"`
	Size     int64  `json:"size"`
	Content  string `json:"content"`
}

// Gist GitHub Gist 元数据。json tag 与 GitHub API 对齐,便于消费方直接
// 序列化落盘备份(下游 BackupGists 即整体 json.MarshalIndent)。
type Gist struct {
	ID          string              `json:"id"`
	Description string              `json:"description"`
	Public      bool                `json:"public"`
	HTMLURL     string              `json:"html_url"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	Files       map[string]GistFile `json:"files"`
}

// GistManager 提供认证用户自己的 gist 列表。可选能力接口:使用前先看
// Provider.Capabilities().Gists(或类型断言)。目前只读——下游备份场景
// 只需要分页列出,创建/修改 gist 不在统一接口范围内。
type GistManager interface {
	// ListMyGists 分页列出 token 用户可见的 gist(GET /gists:已认证调用
	// 返回当前用户的 gist 列表)。page/perPage 语义与其他分页方法一致,
	// 非法值经 NormalizePageOpts 归一。
	ListMyGists(ctx context.Context, page, perPage int) ([]*Gist, error)
}
