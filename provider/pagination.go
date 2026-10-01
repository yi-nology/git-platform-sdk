package provider

import (
	"context"
	"net/http"
	"strconv"
)

const (
	DefaultPage    = 1
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// NormalizePageOpts applies default values for page/perPage.
func NormalizePageOpts(page, perPage int) (int, int) {
	if page <= 0 {
		page = DefaultPage
	}
	if perPage <= 0 {
		perPage = DefaultPerPage
	}
	if perPage > MaxPerPage {
		perPage = MaxPerPage
	}
	return page, perPage
}

// ListAllPages 循环翻页拉取全量：每页调 fetch(page, perPage)，直到某页条数
// < perPage（末页）或达到 maxPages 上限。perPage<=0 取 MaxPerPage，maxPages<=0 取 100。
//
// maxPages 上限是安全阀：若平台忽略 page 参数反复返回整页，"末页截断"永远
// 不会触发，调用方会无限翻页；封顶后返回已取到的数据而不是报错，与平台
// 其余分页辅助的"尽力取全 + 有界"语义一致。每次调用 fetch 前检查 ctx，
// 保证调用方取消后能及时退出而不是白跑完剩余页。
func ListAllPages[T any](ctx context.Context, perPage, maxPages int, fetch func(ctx context.Context, page, perPage int) ([]T, error)) ([]T, error) {
	if perPage <= 0 {
		perPage = MaxPerPage
	}
	if maxPages <= 0 {
		maxPages = 100
	}
	var all []T
	for page := 1; page <= maxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items, err := fetch(ctx, page, perPage)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < perPage {
			return all, nil // 末页：本页不足一页说明没有下一页了
		}
	}
	return all, nil
}

// ParseTotalCountHeader reads X-Total-Count or X-Total from response headers.
// Falls back to the provided default if neither header is present or valid.
func ParseTotalCountHeader(headers http.Header, fallback int) int {
	for _, key := range []string{"X-Total-Count", "X-Total"} {
		if v := headers.Get(key); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				return n
			}
		}
	}
	return fallback
}
